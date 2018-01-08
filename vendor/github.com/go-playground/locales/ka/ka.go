package ka

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ka struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
	percentSuffix          string
	perMille               string
	timeSeparator          string
	inifinity              string
	currencies             []string // idx = enum of currency code
	currencyPositiveSuffix string
	currencyNegativeSuffix string
	monthsAbbreviated      []string
	monthsNarrow           []string
	monthsWide             []string
	daysAbbreviated        []string
	daysNarrow             []string
	daysShort              []string
	daysWide               []string
	periodsAbbreviated     []string
	periodsNarrow          []string
	periodsShort           []string
	periodsWide            []string
	erasAbbreviated        []string
	erasNarrow             []string
	erasWide               []string
	timezones              map[string]string
}

// New returns a new instance of translator for the 'ka' locale
func New() locales.Translator {
	return &ka{
		locale:                 "ka",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{2, 5, 6},
		pluralsRange:           []locales.PluralRule{2, 6},
		decimal:                ",",
		group:                  " ",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "₾", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "იან", "თებ", "მარ", "აპრ", "მაი", "ივნ", "ივლ", "აგვ", "სექ", "ოქტ", "ნოე", "დეკ"},
		monthsNarrow:           []string{"", "ი", "თ", "მ", "ა", "მ", "ი", "ი", "ა", "ს", "ო", "ნ", "დ"},
		monthsWide:             []string{"", "იანვარი", "თებერვალი", "მარტი", "აპრილი", "მაისი", "ივნისი", "ივლისი", "აგვისტო", "სექტემბერი", "ოქტომბერი", "ნოემბერი", "დეკემბერი"},
		daysAbbreviated:        []string{"კვი", "ორშ", "სამ", "ოთხ", "ხუთ", "პარ", "შაბ"},
		daysNarrow:             []string{"კ", "ო", "ს", "ო", "ხ", "პ", "შ"},
		daysShort:              []string{"კვ", "ორ", "სმ", "ოთ", "ხთ", "პრ", "შბ"},
		daysWide:               []string{"კვირა", "ორშაბათი", "სამშაბათი", "ოთხშაბათი", "ხუთშაბათი", "პარასკევი", "შაბათი"},
		periodsAbbreviated:     []string{"AM", "PM"},
		periodsNarrow:          []string{"a", "p"},
		periodsWide:            []string{"AM", "PM"},
		erasAbbreviated:        []string{"ძვ. წ.", "ახ. წ."},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"ძველი წელთაღრიცხვით", "ახალი წელთაღრიცხვით"},
		timezones:              map[string]string{"HNNOMX": "ჩრდილო-დასავლეთ მექსიკის დრო", "ART": "არგენტინის სტანდარტული დრო", "UYT": "ურუგვაის სტანდარტული დრო", "OEZ": "აღმოსავლეთ ევროპის სტანდარტული დრო", "VET": "ვენესუელის დრო", "HNT": "ნიუფაუნდლენდის სტანდარტული დრო", "AKST": "ალასკის სტანდარტული დრო", "AWDT": "დასავლეთ ავსტრალიის ზაფხულის დრო", "MESZ": "ცენტრალური ევროპის ზაფხულის დრო", "IST": "ინდოეთის დრო", "ADT": "ატლანტიკის ოკეანის ზაფხულის დრო", "HKST": "ჰონკონგის ზაფხულის დრო", "GYT": "გაიანის დრო", "GMT": "გრინვიჩის საშუალო დრო", "HNPMX": "მექსიკის წყნარი ოკეანის სტანდარტული დრო", "WIT": "აღმოსავლეთ ინდონეზიის დრო", "WITA": "ცენტრალური ინდონეზიის დრო", "JST": "იაპონიის სტანდარტული დრო", "BT": "ბუტანის დრო", "EDT": "ჩრდილოეთ ამერიკის აღმოსავლეთის ზაფხულის დრო", "ChST": "ჩამოროს დრო", "HEPMX": "მექსიკის წყნარი ოკეანის ზაფხულის დრო", "CHADT": "ჩატემის ზაფხულის დრო", "HEPM": "სენ-პიერის და მიკელონის ზაფხულის დრო", "MST": "MST", "LHDT": "ლორდ-ჰაუს ზაფხულის დრო", "HAT": "ნიუფაუნდლენდის ზაფხულის დრო", "JDT": "იაპონიის ზაფხულის დრო", "ARST": "არგენტინის ზაფხულის დრო", "BOT": "ბოლივიის დრო", "WAT": "დასავლეთ აფრიკის სტანდარტული დრო", "SGT": "სინგაპურის დრო", "HECU": "კუბის ზაფხულის დრო", "CDT": "ჩრდილოეთ ამერიკის ცენტრალური ზაფხულის დრო", "ACWDT": "ცენტრალური და დასავლეთ ავსტრალიის ზაფხულის დრო", "HAST": "ჰავაისა და ალეუტის სტანდარტული დრო", "WARST": "დასავლეთ არგენტინის ზაფხულის დრო", "WEZ": "დასავლეთ ევროპის სტანდარტული დრო", "PST": "ჩრდილოეთ ამერიკის წყნარი ოკეანის სტანდარტული დრო", "HNPM": "სენ-პიერის და მიკელონის სტანდარტული დრო", "MYT": "მალაიზიის დრო", "NZDT": "ახალი ზელანდიის ზაფხულის დრო", "LHST": "ლორდ-ჰაუს სტანდარტული დრო", "HNEG": "აღმოსავლეთ გრენლანდიის სტანდარტული დრო", "HKT": "ჰონკონგის სტანდარტული დრო", "EST": "ჩრდილოეთ ამერიკის აღმოსავლეთის სტანდარტული დრო", "AWST": "დასავლეთ ავსტრალიის სტანდარტული დრო", "UYST": "ურუგვაის ზაფხულის დრო", "HNOG": "დასავლეთ გრენლანდიის სტანდარტული დრო", "ACWST": "ცენტრალური და დასავლეთ ავსტრალიის სტანდარტული დრო", "EAT": "აღმოსავლეთ აფრიკის დრო", "CLST": "ჩილეს ზაფხულის დრო", "CLT": "ჩილეს სტანდარტული დრო", "ACST": "ავსტრალიის ცენტრალური სტანდარტული დრო", "HENOMX": "ჩრდილო-დასავლეთ მექსიკის ზაფხულის დრო", "WAST": "დასავლეთ აფრიკის ზაფხულის დრო", "HEEG": "აღმოსავლეთ გრენლანდიის ზაფხულის დრო", "OESZ": "აღმოსავლეთ ევროპის ზაფხულის დრო", "WART": "დასავლეთ არგენტინის სტანდარტული დრო", "AST": "ატლანტიკის ოკეანის სტანდარტული დრო", "AEST": "აღმოსავლეთ ავსტრალიის სტანდარტული დრო", "HEOG": "დასავლეთ გრენლანდიის ზაფხულის დრო", "CST": "ჩრდილოეთ ამერიკის ცენტრალური სტანდარტული დრო", "MDT": "MDT", "SRT": "სურინამის დრო", "HNCU": "კუბის სტანდარტული დრო", "GFT": "საფრანგეთის გვიანის დრო", "ACDT": "ავსტრალიის ცენტრალური ზაფხულის დრო", "CHAST": "ჩატემის სტანდარტული დრო", "SAST": "სამხრეთ აფრიკის დრო", "COST": "კოლუმბიის ზაფხულის დრო", "ECT": "ეკვადორის დრო", "HADT": "ჰავაისა და ალეუტის ზაფხულის დრო", "NZST": "ახალი ზელანდიის სტანდარტული დრო", "TMT": "თურქმენეთის სტანდარტული დრო", "PDT": "ჩრდილოეთ ამერიკის წყნარი ოკეანის ზაფხულის დრო", "COT": "კოლუმბიის სტანდარტული დრო", "AKDT": "ალასკის ზაფხულის დრო", "CAT": "ცენტრალური აფრიკის დრო", "WESZ": "დასავლეთ ევროპის ზაფხულის დრო", "∅∅∅": "∅∅∅", "MEZ": "ცენტრალური ევროპის სტანდარტული დრო", "AEDT": "აღმოსავლეთ ავსტრალიის ზაფხულის დრო", "TMST": "თურქმენეთის ზაფხულის დრო", "WIB": "დასავლეთ ინდონეზიის დრო"},
	}
}

// Locale returns the current translators string locale
func (ka *ka) Locale() string {
	return ka.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ka'
func (ka *ka) PluralsCardinal() []locales.PluralRule {
	return ka.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ka'
func (ka *ka) PluralsOrdinal() []locales.PluralRule {
	return ka.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ka'
func (ka *ka) PluralsRange() []locales.PluralRule {
	return ka.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ka'
func (ka *ka) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ka'
func (ka *ka) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)
	iMod100 := i % 100

	if i == 1 {
		return locales.PluralRuleOne
	} else if (i == 0) || (iMod100 >= 2 && iMod100 <= 20 && (iMod100 == 40 || iMod100 == 60 || iMod100 == 80)) {
		return locales.PluralRuleMany
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ka'
func (ka *ka) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := ka.CardinalPluralRule(num1, v1)
	end := ka.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOne
	} else if start == locales.PluralRuleOther && end == locales.PluralRuleOne {
		return locales.PluralRuleOther
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ka *ka) MonthAbbreviated(month time.Month) string {
	return ka.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ka *ka) MonthsAbbreviated() []string {
	return ka.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ka *ka) MonthNarrow(month time.Month) string {
	return ka.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ka *ka) MonthsNarrow() []string {
	return ka.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ka *ka) MonthWide(month time.Month) string {
	return ka.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ka *ka) MonthsWide() []string {
	return ka.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ka *ka) WeekdayAbbreviated(weekday time.Weekday) string {
	return ka.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ka *ka) WeekdaysAbbreviated() []string {
	return ka.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ka *ka) WeekdayNarrow(weekday time.Weekday) string {
	return ka.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ka *ka) WeekdaysNarrow() []string {
	return ka.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ka *ka) WeekdayShort(weekday time.Weekday) string {
	return ka.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ka *ka) WeekdaysShort() []string {
	return ka.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ka *ka) WeekdayWide(weekday time.Weekday) string {
	return ka.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ka *ka) WeekdaysWide() []string {
	return ka.daysWide
}

// Decimal returns the decimal point of number
func (ka *ka) Decimal() string {
	return ka.decimal
}

// Group returns the group of number
func (ka *ka) Group() string {
	return ka.group
}

// Group returns the minus sign of number
func (ka *ka) Minus() string {
	return ka.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ka' and handles both Whole and Real numbers based on 'v'
func (ka *ka) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ka.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ka.group) - 1; j >= 0; j-- {
					b = append(b, ka.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ka.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ka' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ka *ka) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ka.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ka.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, ka.percentSuffix...)

	b = append(b, ka.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ka'
func (ka *ka) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ka.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ka.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ka.group) - 1; j >= 0; j-- {
					b = append(b, ka.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ka.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ka.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, ka.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ka'
// in accounting notation.
func (ka *ka) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ka.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ka.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ka.group) - 1; j >= 0; j-- {
					b = append(b, ka.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, ka.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ka.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, ka.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, ka.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ka'
func (ka *ka) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2e}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'ka'
func (ka *ka) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ka.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2e, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ka'
func (ka *ka) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ka.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ka'
func (ka *ka) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ka.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ka.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ka'
func (ka *ka) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ka'
func (ka *ka) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ka'
func (ka *ka) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ka'
func (ka *ka) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ka.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ka.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
