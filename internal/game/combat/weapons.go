package combat

import (
	"encoding/xml"
	"io/ioutil"
	"os"
)

type WeaponType int

const (
	WeaponTypeMelee WeaponType = iota
	WeaponTypeDistance
)

type Weapon struct {
	ID      uint32
	Attack  int
	Defense int
	Range   int
	Type    WeaponType
}

type XMLWeaponMelee struct {
	ID      uint32 `xml:"id,attr"`
	Attack  int    `xml:"attack,attr"`
	Defense int    `xml:"defense,attr"`
	Range   int    `xml:"range,attr"`
}

type XMLWeaponDistance struct {
	ID      uint32 `xml:"id,attr"`
	Attack  int    `xml:"attack,attr"`
	Range   int    `xml:"range,attr"`
}

type XMLWeapons struct {
	Melee    []XMLWeaponMelee    `xml:"melee"`
	Distance []XMLWeaponDistance `xml:"distance"`
}

var weaponsMap = make(map[uint32]*Weapon)

func LoadWeapons(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	var weapons XMLWeapons
	if err := xml.Unmarshal(data, &weapons); err != nil {
		return err
	}

	for _, w := range weapons.Melee {
		rng := w.Range
		if rng <= 0 {
			rng = 1
		}
		weaponsMap[w.ID] = &Weapon{
			ID:      w.ID,
			Attack:  w.Attack,
			Defense: w.Defense,
			Range:   rng,
			Type:    WeaponTypeMelee,
		}
	}

	for _, w := range weapons.Distance {
		rng := w.Range
		if rng <= 0 {
			rng = 5
		}
		weaponsMap[w.ID] = &Weapon{
			ID:      w.ID,
			Attack:  w.Attack,
			Defense: 0,
			Range:   rng,
			Type:    WeaponTypeDistance,
		}
	}

	return nil
}

func GetWeapon(id uint32) *Weapon {
	return weaponsMap[id]
}
