package tools

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"
)

func BytesToUInt64(buf []byte) uint64 {
	return binary.BigEndian.Uint64(buf)
}

var random *rand.Rand

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func RandInt(n int) int {
	return random.Intn(n)
}

func RandIntn(i int) int {
	return random.Intn(i)
}

func ParseInt64(value string) (int64, error) {
	ret, err := strconv.ParseInt(value, 10, 64)
	return ret, err
}

func ParseInt(value string) (int, error) {
	ret, err := strconv.Atoi(value)
	return ret, err
}

func Int2String(val int64) string {
	return strconv.FormatInt(val, 10)
}

func ToJson(val interface{}) string {
	bs, err := json.Marshal(val)
	if err == nil {
		return string(bs)
	}
	return ""
}

func BoolPtr(f bool) *bool {
	return &f
}

func IntPtr(i int) *int {
	return &i
}

func Int64Ptr(i int64) *int64 {
	return &i
}

func String2Int64(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}

func ToInt(str string) int {
	intVal, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}
	return int(intVal)
}

func PbMarshal(obj proto.Message) ([]byte, error) {
	bytes, err := proto.Marshal(obj)
	return bytes, err
}
func PbUnMarshal(bytes []byte, typeScope proto.Message) error {
	err := proto.Unmarshal(bytes, typeScope)
	return err
}

func JsonMarshal(obj interface{}) ([]byte, error) {
	return json.Marshal(obj)
}

func JsonUnMarshal(bytes []byte, obj interface{}) error {
	return json.Unmarshal(bytes, obj)
}

func SHA1(s string) string {
	o := sha1.New()
	o.Write([]byte(s))
	return hex.EncodeToString(o.Sum(nil))
}
