package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	newInboxStorageForWhatsAppWebhook = storages.NewInboxStorage
	getImSdkForWhatsAppWebhook        = getImSdkForCustomer
	sendWhatsAppGroupMsg              = func(sdk *juggleimsdk.JuggleIMSdk, msg juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		return sdk.SendGroupMsg(msg)
	}
)

type WhatsAppWebhookPayload struct {
	Object string                 `json:"object"`
	Entry  []WhatsAppWebhookEntry `json:"entry"`
}

type WhatsAppWebhookEntry struct {
	Changes []WhatsAppWebhookChange `json:"changes"`
}

type WhatsAppWebhookChange struct {
	Field string               `json:"field"`
	Value WhatsAppWebhookValue `json:"value"`
}

type WhatsAppWebhookValue struct {
	Metadata WhatsAppMetadata  `json:"metadata"`
	Contacts []WhatsAppContact `json:"contacts"`
	Messages []WhatsAppMessage `json:"messages"`
	Statuses []WhatsAppStatus  `json:"statuses"`
}

type WhatsAppMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type WhatsAppContact struct {
	WaID    string `json:"wa_id"`
	UserID  string `json:"user_id"`
	Profile struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"profile"`
}

type WhatsAppStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type WhatsAppMessage struct {
	From       string `json:"from"`
	FromUserID string `json:"from_user_id"`
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Type       string `json:"type"`
	Text       *struct {
		Body string `json:"body"`
	} `json:"text"`
	Image    *WhatsAppMedia        `json:"image"`
	Audio    *WhatsAppMedia        `json:"audio"`
	Video    *WhatsAppMedia        `json:"video"`
	Document *WhatsAppMedia        `json:"document"`
	Sticker  *WhatsAppMedia        `json:"sticker"`
	Location *WhatsAppLocation     `json:"location"`
	Contacts []WhatsAppContactCard `json:"contacts"`
	Button   *struct {
		Text    string `json:"text"`
		Payload string `json:"payload"`
	} `json:"button"`
	Interactive *WhatsAppInteractive `json:"interactive"`
	Reaction    *struct {
		Emoji string `json:"emoji"`
	} `json:"reaction"`
	Errors []struct {
		Code  int    `json:"code"`
		Title string `json:"title"`
	} `json:"errors"`
}

type WhatsAppMedia struct {
	ID       string `json:"id"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
}

type WhatsAppLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	URL       string  `json:"url"`
}

type WhatsAppContactCard struct {
	Name struct {
		FormattedName string `json:"formatted_name"`
		FirstName     string `json:"first_name"`
		LastName      string `json:"last_name"`
	} `json:"name"`
	Phones []struct {
		Phone string `json:"phone"`
	} `json:"phones"`
}

type WhatsAppInteractive struct {
	Type        string `json:"type"`
	ButtonReply *struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"button_reply"`
	ListReply *struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"list_reply"`
}

func ProcessWhatsAppWebhook(inboxID string, body []byte, signature string) errs.IMErrorCode {
	inbox, code := resolveWhatsAppWebhookInbox(inboxID)
	if code != errs.IMErrorCode_SUCCESS {
		return code
	}
	conf := ParseWhatsAppChannelConf(inbox.ChannelConf)
	if !verifyWhatsAppSignature(body, signature, conf.AppSecret) {
		log.Printf("[WhatsAppWebhook] invalid signature inbox=%s", inboxID)
		return errs.IMErrorCode_APP_NOT_LOGIN
	}

	var payload WhatsAppWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[WhatsAppWebhook] invalid payload inbox=%s err=%v", inboxID, err)
		return errs.IMErrorCode_SUCCESS
	}
	if payload.Object != "" && payload.Object != "whatsapp_business_account" {
		return errs.IMErrorCode_SUCCESS
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			value := change.Value
			if value.Metadata.PhoneNumberID != "" &&
				conf.PhoneNumberID != "" &&
				value.Metadata.PhoneNumberID != conf.PhoneNumberID {
				log.Printf("[WhatsAppWebhook] phone number mismatch inbox=%s got=%s want=%s",
					inboxID, value.Metadata.PhoneNumberID, conf.PhoneNumberID)
				continue
			}
			for _, message := range value.Messages {
				if code := processWhatsAppMessage(inbox, value.Contacts, message); code != errs.IMErrorCode_SUCCESS {
					return code
				}
			}
		}
	}
	return errs.IMErrorCode_SUCCESS
}

func ValidateWhatsAppWebhookSignature(inboxID string, body []byte, signature string) bool {
	inbox, code := resolveWhatsAppWebhookInbox(inboxID)
	if code != errs.IMErrorCode_SUCCESS {
		return false
	}
	return verifyWhatsAppSignature(body, signature, ParseWhatsAppChannelConf(inbox.ChannelConf).AppSecret)
}

func WhatsAppWebhookChallenge(inboxID, mode, verifyToken, challenge string) (string, bool) {
	if strings.TrimSpace(mode) != "subscribe" || strings.TrimSpace(challenge) == "" {
		return "", false
	}
	inbox, code := resolveWhatsAppWebhookInbox(inboxID)
	if code != errs.IMErrorCode_SUCCESS {
		return "", false
	}
	conf := ParseWhatsAppChannelConf(inbox.ChannelConf)
	if strings.TrimSpace(conf.WebhookVerifyToken) == "" || !hmac.Equal(
		[]byte(conf.WebhookVerifyToken),
		[]byte(strings.TrimSpace(verifyToken)),
	) {
		return "", false
	}
	return challenge, true
}

func resolveWhatsAppWebhookInbox(inboxID string) (*storageModels.Inbox, errs.IMErrorCode) {
	inboxID = strings.TrimSpace(inboxID)
	if inboxID == "" {
		return nil, errs.IMErrorCode_APP_ParamError
	}
	inboxes, err := newInboxStorageForWhatsAppWebhook().FindByInboxIdAny(inboxID)
	if err != nil {
		return nil, errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if len(inboxes) != 1 || inboxes[0] == nil {
		return nil, errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	inbox := inboxes[0]
	if inbox.ChannelType != string(ChannelType_WhatsApp) {
		return nil, errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return inbox, errs.IMErrorCode_SUCCESS
}

func processWhatsAppMessage(inbox *storageModels.Inbox, contacts []WhatsAppContact, message WhatsAppMessage) errs.IMErrorCode {
	if inbox == nil || strings.TrimSpace(message.ID) == "" {
		return errs.IMErrorCode_SUCCESS
	}
	identity := strings.TrimSpace(message.From)
	if identity == "" {
		identity = strings.TrimSpace(message.FromUserID)
	}
	if identity == "" {
		return errs.IMErrorCode_SUCCESS
	}
	content := whatsappMessageContent(message)
	if content == "" {
		return errs.IMErrorCode_SUCCESS
	}

	resultCode, result := startCustomerTicket(customerTicketStartReq{
		AppKey:      inbox.AppKey,
		InboxId:     inbox.InboxId,
		Identifier:  identity,
		Nickname:    whatsappContactName(contacts, identity),
		ChannelType: ChannelType_WhatsApp,
		GenerateSourceId: func() string {
			return CustomerSourceIDPrefix + tools.GenerateUUIDShort22()
		},
	})
	if resultCode != errs.IMErrorCode_SUCCESS {
		return resultCode
	}
	sdk := getImSdkForWhatsAppWebhook(inbox.AppKey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED
	}
	imMessageID := fmt.Sprintf("whatsapp:%s:%s", inbox.InboxId, strings.TrimSpace(message.ID))
	sendCode, _, err := sendWhatsAppGroupMsg(sdk, juggleimsdk.Message{
		SenderId:   result.Rel.SourceId,
		TargetId:   result.Ticket.TicketId,
		MsgType:    "jg:text",
		MsgContent: mustJSON(map[string]string{"content": content}),
		MsgId:      &imMessageID,
		IsStorage:  boolPtrForWhatsApp(true),
		IsCount:    boolPtrForWhatsApp(true),
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if sendCode != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(sendCode)
	}
	return errs.IMErrorCode_SUCCESS
}

func whatsappMessageContent(message WhatsAppMessage) string {
	switch strings.ToLower(strings.TrimSpace(message.Type)) {
	case "text":
		if message.Text != nil {
			return strings.TrimSpace(message.Text.Body)
		}
	case "image":
		return mediaContent("image", message.Image)
	case "audio":
		return mediaContent("audio", message.Audio)
	case "video":
		return mediaContent("video", message.Video)
	case "document":
		return mediaContent("document", message.Document)
	case "sticker":
		return mediaContent("sticker", message.Sticker)
	case "location":
		if message.Location != nil {
			name := strings.TrimSpace(message.Location.Name)
			address := strings.TrimSpace(message.Location.Address)
			label := strings.TrimSpace(strings.Join([]string{name, address}, ", "))
			if label != "" {
				return fmt.Sprintf("Location: %s (%.6f, %.6f)", label, message.Location.Latitude, message.Location.Longitude)
			}
			return fmt.Sprintf("Location: %.6f, %.6f", message.Location.Latitude, message.Location.Longitude)
		}
	case "contacts":
		var names []string
		for _, contact := range message.Contacts {
			name := strings.TrimSpace(contact.Name.FormattedName)
			if name == "" {
				name = strings.TrimSpace(strings.Join([]string{contact.Name.FirstName, contact.Name.LastName}, " "))
			}
			if name != "" {
				names = append(names, name)
			}
		}
		return strings.TrimSpace(strings.Join(names, ", "))
	case "button":
		if message.Button != nil {
			return strings.TrimSpace(message.Button.Text)
		}
	case "interactive":
		if message.Interactive != nil {
			if message.Interactive.ButtonReply != nil {
				return strings.TrimSpace(message.Interactive.ButtonReply.Title)
			}
			if message.Interactive.ListReply != nil {
				return strings.TrimSpace(message.Interactive.ListReply.Title)
			}
		}
	case "reaction":
		if message.Reaction != nil {
			return strings.TrimSpace(message.Reaction.Emoji)
		}
	case "unsupported":
		return "WhatsApp message is unavailable."
	}
	return ""
}

func mediaContent(kind string, media *WhatsAppMedia) string {
	if media == nil {
		return ""
	}
	if caption := strings.TrimSpace(media.Caption); caption != "" {
		return caption
	}
	if media.Filename != "" {
		return fmt.Sprintf("WhatsApp %s: %s", kind, media.Filename)
	}
	return fmt.Sprintf("WhatsApp %s message", kind)
}

func whatsappContactName(contacts []WhatsAppContact, identity string) string {
	for _, contact := range contacts {
		if contact.WaID != identity && contact.UserID != identity {
			continue
		}
		if name := strings.TrimSpace(contact.Profile.Name); name != "" {
			return name
		}
		if username := strings.TrimSpace(contact.Profile.Username); username != "" {
			return username
		}
	}
	return identity
}

func verifyWhatsAppSignature(body []byte, signature, appSecret string) bool {
	appSecret = strings.TrimSpace(appSecret)
	if appSecret == "" {
		return true
	}
	signature = strings.TrimSpace(signature)
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	expected := hmac.New(sha256.New, []byte(appSecret))
	_, _ = expected.Write(body)
	actual, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	return hmac.Equal(actual, expected.Sum(nil))
}

func mustJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func boolPtrForWhatsApp(value bool) *bool {
	return &value
}
