# WhatsApp Cloud API Integration

## Create an Inbox

In the console, create a `WhatsApp` inbox with:

- `Phone Number ID`: the Meta WhatsApp phone number ID.
- `Access Token`: a token allowed to read the phone number and send messages.
- `Webhook Verify Token`: an arbitrary secret shared with the Meta webhook setup.
- `App Secret`: optional, but recommended. When present, POST requests must include a valid `X-Hub-Signature-256`.
- `API Base URL` and `API Version`: optional overrides for compatible Graph API deployments.

The server validates the access token and Phone Number ID before persisting the inbox. Secrets are stored in the channel configuration but are omitted from console responses.

## Configure the Meta Webhook

Use the following callback URL:

```text
{jmateBaseUrl}/jmate/webhooks/whatsapp/{inboxId}
```

The GET endpoint answers the Meta verification challenge with the configured Webhook Verify Token. Subscribe the WhatsApp `messages` field. The POST endpoint accepts inbound messages and delivery status events; status events are ignored by the current JuggleIM bridge.

## Message Flow

1. WhatsApp sends a message to the webhook.
2. JuggleMate finds or creates the customer and ticket for the WhatsApp sender.
3. The message is posted to the ticket's JuggleIM group with a deterministic WhatsApp message ID.
4. Agent or bot text messages received through `/jmate/webhooks/message` are sent to the customer's WhatsApp number.

Inbound text, media captions, locations, contacts, button replies, list replies, and reactions are represented as text in the JuggleIM ticket. Outbound media/template messages are not enabled by this adapter yet.
