package vocations

import (
	"encoding/xml"
	"io/ioutil"
	"os"
)

type VocationFormula struct {
	MeleeDamage float64 `xml:"meleeDamage,attr"`
	DistDamage  float64 `xml:"distDamage,attr"`
	Defense     float64 `xml:"defense,attr"`
	Armor       float64 `xml:"armor,attr"`
}

type VocationSkill struct {
	ID         int     `xml:"id,attr"`
	Multiplier float64 `xml:"multiplier,attr"`
}

type Vocation struct {
	ID             uint32          `xml:"id,attr"`
	Name           string          `xml:"name,attr"`
	GainHPTicks    int             `xml:"gainhpticks,attr"`
	GainHPAmount   int             `xml:"gainhpamount,attr"`
	GainManaTicks  int             `xml:"gainmanaticks,attr"`
	GainManaAmount int             `xml:"gainmanaamount,attr"`
	AttackSpeed    int             `xml:"attackspeed,attr"`
	ManaMultiplier float64         `xml:"manamultiplier,attr"`
	BaseSpeed      int             `xml:"basespeed,attr"`
	Formula        VocationFormula `xml:"formula"`
	Skills         []VocationSkill `xml:"skill"`
}

type VocationsList struct {
	Vocations []Vocation `xml:"vocation"`
}

var vocationsMap = make(map[uint32]*Vocation)

func LoadVocations(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	var vList VocationsList
	if err := xml.Unmarshal(data, &vList); err != nil {
		return err
	}

	for i := range vList.Vocations {
		voc := &vList.Vocations[i]
		vocationsMap[voc.ID] = voc
	}

	return nil
}

func GetVocation(id uint32) *Vocation {
	return vocationsMap[id]
}
