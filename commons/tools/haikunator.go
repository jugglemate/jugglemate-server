package tools

import "fmt"

var haikunatorAdjectives = []string{
	"autumn",
	"bold",
	"calm",
	"cool",
	"dawn",
	"dry",
	"fancy",
	"floral",
	"gentle",
	"green",
	"hidden",
	"icy",
	"late",
	"lively",
	"misty",
	"orange",
	"plain",
	"quiet",
	"red",
	"rough",
	"shy",
	"small",
	"soft",
	"spring",
	"still",
	"summer",
	"wild",
	"winter",
}

var haikunatorNouns = []string{
	"bird",
	"brook",
	"cloud",
	"dream",
	"field",
	"fire",
	"flower",
	"forest",
	"frog",
	"glade",
	"hill",
	"lake",
	"leaf",
	"meadow",
	"moon",
	"morning",
	"mountain",
	"pine",
	"pond",
	"rain",
	"river",
	"sea",
	"shadow",
	"sky",
	"snow",
	"star",
	"sun",
	"wave",
}

func GenerateHaikunator() string {
	adjective := haikunatorAdjectives[RandInt(len(haikunatorAdjectives))]
	noun := haikunatorNouns[RandInt(len(haikunatorNouns))]
	return fmt.Sprintf("%s-%s-%04d", adjective, noun, RandInt(10000))
}
