package eu_ES

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type eu_ES struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
	percentPrefix          string
	perMille               string
	timeSeparator          string
	inifinity              string
	currencies             []string // idx = enum of currency code
	currencyPositiveSuffix string
	currencyNegativePrefix string
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

// New returns a new instance of translator for the 'eu_ES' locale
func New() locales.Translator {
	return &eu_ES{
		locale:                 "eu_ES",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{6},
		pluralsRange:           []locales.PluralRule{6},
		decimal:                ",",
		group:                  ".",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentPrefix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: " )",
		monthsAbbreviated:      []string{"", "urt.", "ots.", "mar.", "api.", "mai.", "eka.", "uzt.", "abu.", "ira.", "urr.", "aza.", "abe."},
		monthsNarrow:           []string{"", "U", "O", "M", "A", "M", "E", "U", "A", "I", "U", "A", "A"},
		monthsWide:             []string{"", "urtarrila", "otsaila", "martxoa", "apirila", "maiatza", "ekaina", "uztaila", "abuztua", "iraila", "urria", "azaroa", "abendua"},
		daysAbbreviated:        []string{"ig.", "al.", "ar.", "az.", "og.", "or.", "lr."},
		daysNarrow:             []string{"I", "A", "A", "A", "O", "O", "L"},
		daysShort:              []string{"ig.", "al.", "ar.", "az.", "og.", "or.", "lr."},
		daysWide:               []string{"igandea", "astelehena", "asteartea", "asteazkena", "osteguna", "ostirala", "larunbata"},
		periodsAbbreviated:     []string{"AM", "PM"},
		periodsWide:            []string{"AM", "PM"},
		erasAbbreviated:        []string{"K.a.", "K.o."},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"K.a.", "Kristo ondoren"},
		timezones:              map[string]string{"UYT": "Uruguayko ordu estandarra", "IST": "Indiako ordua", "HEEG": "Groenlandia ekialdeko udako ordutegia", "ACST": "Australia erdialdeko ordu estandarra", "HNPMX": "Mexikoko Pazifikoko ordutegi estandarra", "AWST": "Australia mendebaldeko ordu estandarra", "HNOG": "Groenlandia mendebaldeko ordutegi estandarra", "COT": "Kolonbiako ordu estandarra", "EST": "Ipar Amerikako ekialdeko ordutegi estandarra", "GMT": "Greenwich meridianoko ordua", "OESZ": "Europa ekialdeko udako ordua", "HAST": "Hawaii-Aleutiar uharteetako ordutegi estandarra", "OEZ": "Europa ekialdeko ordu estandarra", "AST": "Atlantikoko ordutegi estandarra", "WAST": "Afrika mendebaldeko udako ordua", "HNT": "Ternuako ordutegi estandarra", "HEPMX": "Mexikoko Pazifikoko udako ordutegia", "MEZ": "Europa erdialdeko ordu estandarra", "HNNOMX": "Mexikoko ipar-ekialdeko ordutegi estandarra", "JDT": "Japoniako udako ordua", "ARST": "Argentinako udako ordutegia", "SAST": "Afrika hegoaldeko ordua", "ChST": "Chamorroko ordu estandarra", "PDT": "Ipar Amerikako Pazifikoko udako ordutegia", "MDT": "MDT", "HKT": "Hong Kongo ordu estandarra", "CLT": "Txileko ordu estandarra", "GYT": "Guyanako ordua", "AEDT": "Australia ekialdeko udako ordua", "WIT": "Indonesia ekialdeko ordua", "WART": "Argentina mendebaldeko ordutegia estandarra", "ADT": "Atlantikoko udako ordutegia", "CLST": "Txileko udako ordua", "EDT": "Ipar Amerikako ekialdeko udako ordutegia", "CHADT": "Chathamgo udako ordua", "HEPM": "Saint-Pierre eta Mikeluneko udako ordutegia", "AWDT": "Australia mendebaldeko udako ordua", "NZST": "Zeelanda Berriko ordu estandarra", "JST": "Japoniako ordu estandarra", "WAT": "Afrika mendebaldeko ordu estandarra", "GFT": "Guyana Frantseseko ordua", "HKST": "Hong Kongo udako ordua", "∅∅∅": "∅∅∅", "LHST": "Lord Howeko ordu estandarra", "WARST": "Argentina mendebaldeko udako ordutegia", "HEOG": "Groenlandia mendebaldeko udako ordutegia", "EAT": "Afrika ekialdeko ordua", "HENOMX": "Mexikoko ipar-ekialdeko udako ordutegia", "WEZ": "Europa mendebaldeko ordu estandarra", "WESZ": "Europa mendebaldeko udako ordua", "AEST": "Australia ekialdeko ordu estandarra", "HADT": "Hawaii-Aleutiar uharteetako udako ordutegia", "ACDT": "Australia erdialdeko udako ordua", "ECT": "Ekuadorreko ordua", "WIB": "Indonesia mendebaldeko ordua", "CST": "Ipar Amerikako erdialdeko ordu estandarra", "TMT": "Turkmenistango ordu estandarra", "HAT": "Ternuako udako ordutegia", "SGT": "Singapurreko ordu estandarra", "MST": "MST", "ACWDT": "Australia erdi-mendebaldeko udako ordua", "TMST": "Turkmenistango udako ordua", "ART": "Argentinako ordutegi estandarra", "HNEG": "Groenlandia ekialdeko ordutegi estandarra", "AKDT": "Alaskako udako ordutegia", "PST": "Ipar Amerikako Pazifikoko ordutegi estandarra", "CHAST": "Chathamgo ordu estandarra", "BT": "Bhutango ordua", "CDT": "Ipar Amerikako erdialdeko udako ordutegia", "BOT": "Boliviako ordutegia", "MESZ": "Europa erdialdeko udako ordua", "WITA": "Indonesia erdialdeko ordua", "LHDT": "Lord Howeko udako ordua", "COST": "Kolonbiako udako ordua", "AKST": "Alaskako ordutegi estandarra", "HNPM": "Saint-Pierre eta Mikeluneko ordutegi estandarra", "ACWST": "Australia erdi-mendebaldeko ordu estandarra", "VET": "Venezuelako ordua", "CAT": "Afrika erdialdeko ordua", "HNCU": "Kubako ordutegi estandarra", "UYST": "Uruguayko udako ordua", "MYT": "Malaysiako ordua", "NZDT": "Zeelanda Berriko udako ordua", "HECU": "Kubako udako ordutegia", "SRT": "Surinamgo ordua"},
	}
}

// Locale returns the current translators string locale
func (eu *eu_ES) Locale() string {
	return eu.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'eu_ES'
func (eu *eu_ES) PluralsCardinal() []locales.PluralRule {
	return eu.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'eu_ES'
func (eu *eu_ES) PluralsOrdinal() []locales.PluralRule {
	return eu.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'eu_ES'
func (eu *eu_ES) PluralsRange() []locales.PluralRule {
	return eu.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'eu_ES'
func (eu *eu_ES) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'eu_ES'
func (eu *eu_ES) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'eu_ES'
func (eu *eu_ES) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (eu *eu_ES) MonthAbbreviated(month time.Month) string {
	return eu.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (eu *eu_ES) MonthsAbbreviated() []string {
	return eu.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (eu *eu_ES) MonthNarrow(month time.Month) string {
	return eu.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (eu *eu_ES) MonthsNarrow() []string {
	return eu.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (eu *eu_ES) MonthWide(month time.Month) string {
	return eu.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (eu *eu_ES) MonthsWide() []string {
	return eu.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (eu *eu_ES) WeekdayAbbreviated(weekday time.Weekday) string {
	return eu.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (eu *eu_ES) WeekdaysAbbreviated() []string {
	return eu.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (eu *eu_ES) WeekdayNarrow(weekday time.Weekday) string {
	return eu.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (eu *eu_ES) WeekdaysNarrow() []string {
	return eu.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (eu *eu_ES) WeekdayShort(weekday time.Weekday) string {
	return eu.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (eu *eu_ES) WeekdaysShort() []string {
	return eu.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (eu *eu_ES) WeekdayWide(weekday time.Weekday) string {
	return eu.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (eu *eu_ES) WeekdaysWide() []string {
	return eu.daysWide
}

// Decimal returns the decimal point of number
func (eu *eu_ES) Decimal() string {
	return eu.decimal
}

// Group returns the group of number
func (eu *eu_ES) Group() string {
	return eu.group
}

// Group returns the minus sign of number
func (eu *eu_ES) Minus() string {
	return eu.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'eu_ES' and handles both Whole and Real numbers based on 'v'
func (eu *eu_ES) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, eu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, eu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, eu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'eu_ES' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (eu *eu_ES) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, eu.decimal[0])
			inWhole = true

			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, eu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, eu.minus[0])
	}

	for j := len(eu.percentPrefix) - 1; j >= 0; j-- {
		b = append(b, eu.percentPrefix[j])
	}

	b = append(b, eu.percent[0])

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'eu_ES'
func (eu *eu_ES) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := eu.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, eu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, eu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, eu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, eu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, eu.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'eu_ES'
// in accounting notation.
func (eu *eu_ES) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := eu.currencies[currency]
	l := len(s) + len(symbol) + 6 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, eu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, eu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, eu.currencyNegativePrefix[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, eu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, eu.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, eu.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, eu.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x28, 0x65}...)
	b = append(b, []byte{0x29, 0x6b, 0x6f}...)
	b = append(b, []byte{0x20}...)
	b = append(b, eu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x28, 0x65}...)
	b = append(b, []byte{0x29, 0x6b, 0x6f}...)
	b = append(b, []byte{0x20}...)
	b = append(b, eu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, eu.daysWide[t.Weekday()]...)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20, 0x28}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	b = append(b, []byte{0x29}...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'eu_ES'
func (eu *eu_ES) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, eu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20, 0x28}...)

	tz, _ := t.Zone()

	if btz, ok := eu.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	b = append(b, []byte{0x29}...)

	return string(b)
}
