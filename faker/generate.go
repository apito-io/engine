package faker

import (
	"github.com/brianvoe/gofakeit/v6"
	"math"
	"strings"
	faker2 "syreclabs.com/go/faker"
	"time"
)

func StringFormatter(val string) string {
	// else if faker then format it
	fakeKit := gofakeit.NewCrypto()
	name := strings.TrimSpace(strings.Split(val, "|")[0])
	switch name {
	case "product_name":
		return faker2.Commerce().ProductName()
	case "product_category":
		return faker2.Commerce().Department()
	case "color":
		return fakeKit.Color()
	case "time":
		return fakeKit.Date().Format(time.Kitchen)
	case "name":
		return fakeKit.Name()
	case "name_prefix":
		return fakeKit.NamePrefix()
	case "name_suffix":
		return fakeKit.NameSuffix()
	case "first_name":
		return fakeKit.FirstName()
	case "last_name":
		return fakeKit.LastName()
	case "gender":
		return fakeKit.Gender()
	case "ssn":
		return fakeKit.SSN()
	case "email":
		return fakeKit.Email()
	case "phone":
		return fakeKit.Phone()
	case "city":
		return fakeKit.Address().City
	case "country":
		return fakeKit.Country()
	case "state":
		return fakeKit.State()
	case "street":
		return fakeKit.Street()
	case "zip":
		return fakeKit.Zip()
	case "quote":
		return fakeKit.Quote()
	case "phase":
		return fakeKit.Phrase()
	case "question":
		return fakeKit.Question()
	case "sentence":
		return fakeKit.Sentence(7)
	case "uuid":
		return fakeKit.UUID()
	case "url":
		return fakeKit.URL()
	case "credit_card_number":
		return fakeKit.CreditCard().Number
	case "company":
		return fakeKit.Company()
	case "hacker_phrase":
		return fakeKit.HackerPhrase()
	case "hipstar_sentence":
		return fakeKit.HipsterSentence(7)
	case "date":
		return fakeKit.Date().UTC().Format("2006-01-02")
	case "breakfast":
		return fakeKit.Breakfast()
	case "dessert":
		return fakeKit.Dessert()
	case "dinner":
		return fakeKit.Dinner()
	case "lunch":
		return fakeKit.Lunch()
	case "fruit":
		return fakeKit.Fruit()
	case "animal":
		return fakeKit.Animal()
	case "car_model":
		return fakeKit.CarModel()
	case "job":
		return fakeKit.JobTitle()
	}
	return val
}

func MultilineFormatter(isFaker bool, val string) string {
	if !isFaker {
		return val
	}
	// else if faker then format it
	fakeKit := gofakeit.NewCrypto()
	name := strings.TrimSpace(strings.Split(val, "|")[0])
	switch name {
	case "paragraph":
		return fakeKit.Paragraph(2, 5, 20, `\n`)
	case "lorem_paragraph":
		return fakeKit.LoremIpsumParagraph(2, 5, 20, `\n`)
	case "hipstar_paragraph":
		return fakeKit.HipsterParagraph(2, 5, 20, `\n`)
	}
	return val
}

func ListFormatter(isFaker bool, isMultiChoice bool, randomList []string, val interface{}) interface{} {
	if !isFaker {
		return val
	}
	if len(randomList) > 0 {
		if isMultiChoice {
			return randomList
		} else {
			return randomList[0]
		}
	}
	var list []string
	for i:=0; i < 4; i++ {
		fake := StringFormatter(val.(string))
		list = append(list, fake)
	}
	return list
}

func roundTo(n float64, decimals uint32) float64 {
	return math.Round(n*math.Pow(10, float64(decimals))) / math.Pow(10, float64(decimals))
}

func NumberFormatter(val interface{}) interface{} {
	// else if faker then format it
	fakeKit := gofakeit.NewCrypto()
	name := strings.TrimSpace(strings.Split(val.(string), "|")[0])
	switch name {
	case "int":
		v := fakeKit.Uint32()
		return v
	case "double":
		v := fakeKit.Float64Range(100, 999)
		return v
	case "price":
		v := fakeKit.Price(2, 100)
		return v
	case "rating5":
		return roundTo(fakeKit.Float64Range(1, 5), 2)
	case "rating10":
		return roundTo(fakeKit.Float64Range(1, 10), 2)
	}
	return val
}

func GeoFormatter(val interface{}) interface{} {
	// else if faker then format it
	fakeKit := gofakeit.NewCrypto()
	lat := fakeKit.Latitude()
	lon := fakeKit.Longitude()
	v := map[string]interface{}{
		"lat":         lat,
		"lon":         lon,
		"type":        "Point",
		"coordinates": []float64{lat, lon},
	}
	return v
}
