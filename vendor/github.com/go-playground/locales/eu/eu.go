package eu

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type eu struct {
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

// New returns a new instance of translator for the 'eu' locale
func New() locales.Translator {
	return &eu{
		locale:                 "eu",
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
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "A$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "₧", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JP¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
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
		timezones:              map[string]string{"CHADT": "Chathamgo udako ordua", "HNNOMX": "Mexikoko ipar-ekialdeko ordutegi estandarra", "LHST": "Lord Howeko ordu estandarra", "ARST": "Argentinako udako ordutegia", "EDT": "Ipar Amerikako ekialdeko udako ordutegia", "HEEG": "Groenlandia ekialdeko udako ordutegia", "HNT": "Ternuako ordutegi estandarra", "COST": "Kolonbiako udako ordua", "AST": "Atlantikoko ordutegi estandarra", "HNEG": "Groenlandia ekialdeko ordutegi estandarra", "HKT": "Hong Kongo ordu estandarra", "ECT": "Ekuadorreko ordua", "CHAST": "Chathamgo ordu estandarra", "HNPM": "Saint-Pierre eta Mikeluneko ordutegi estandarra", "MEZ": "Europa erdialdeko ordu estandarra", "WITA": "Indonesia erdialdeko ordua", "ART": "Argentinako ordutegi estandarra", "HEOG": "Groenlandia mendebaldeko udako ordutegia", "CLT": "Txileko ordu estandarra", "CST": "Ipar Amerikako erdialdeko ordu estandarra", "SRT": "Surinamgo ordua", "ACWST": "Australia erdi-mendebaldeko ordu estandarra", "NZDT": "Zeelanda Berriko udako ordua", "TMT": "Turkmenistango ordu estandarra", "ADT": "Atlantikoko udako ordutegia", "WAT": "Afrika mendebaldeko ordu estandarra", "OEZ": "Europa ekialdeko ordu estandarra", "BT": "Bhutango ordua", "JST": "Japoniako ordu estandarra", "ACDT": "Australia erdialdeko udako ordua", "BOT": "Boliviako ordutegia", "UYST": "Uruguayko udako ordua", "HKST": "Hong Kongo udako ordua", "EST": "Ipar Amerikako ekialdeko ordutegi estandarra", "MDT": "MDT", "MESZ": "Europa erdialdeko udako ordua", "EAT": "Afrika ekialdeko ordua", "AKST": "Alaskako ordutegi estandarra", "PDT": "Ipar Amerikako Pazifikoko udako ordutegia", "MYT": "Malaysiako ordua", "WARST": "Argentina mendebaldeko udako ordutegia", "COT": "Kolonbiako ordu estandarra", "HNPMX": "Mexikoko Pazifikoko ordutegi estandarra", "AWDT": "Australia mendebaldeko udako ordua", "GFT": "Guyana Frantseseko ordua", "GMT": "Greenwich meridianoko ordua", "AEST": "Australia ekialdeko ordu estandarra", "LHDT": "Lord Howeko udako ordua", "HEPM": "Saint-Pierre eta Mikeluneko udako ordutegia", "CDT": "Ipar Amerikako erdialdeko udako ordutegia", "NZST": "Zeelanda Berriko ordu estandarra", "TMST": "Turkmenistango udako ordua", "IST": "Indiako ordua", "AEDT": "Australia ekialdeko udako ordua", "HAT": "Ternuako udako ordutegia", "GYT": "Guyanako ordua", "AKDT": "Alaskako udako ordutegia", "ACST": "Australia erdialdeko ordu estandarra", "SGT": "Singapurreko ordu estandarra", "PST": "Ipar Amerikako Pazifikoko ordutegi estandarra", "HNOG": "Groenlandia mendebaldeko ordutegi estandarra", "WAST": "Afrika mendebaldeko udako ordua", "VET": "Venezuelako ordua", "HAST": "Hawaii-Aleutiar uharteetako ordutegi estandarra", "JDT": "Japoniako udako ordua", "OESZ": "Europa ekialdeko udako ordua", "∅∅∅": "Azoreetako udako ordua", "ACWDT": "Australia erdi-mendebaldeko udako ordua", "WESZ": "Europa mendebaldeko udako ordua", "ChST": "Chamorroko ordu estandarra", "WIT": "Indonesia ekialdeko ordua", "HADT": "Hawaii-Aleutiar uharteetako udako ordutegia", "WART": "Argentina mendebaldeko ordutegia estandarra", "SAST": "Afrika hegoaldeko ordua", "CAT": "Afrika erdialdeko ordua", "HNCU": "Kubako ordutegi estandarra", "HECU": "Kubako udako ordutegia", "MST": "MST", "AWST": "Australia mendebaldeko ordu estandarra", "UYT": "Uruguayko ordu estandarra", "HENOMX": "Mexikoko ipar-ekialdeko udako ordutegia", "HEPMX": "Mexikoko Pazifikoko udako ordutegia", "WIB": "Indonesia mendebaldeko ordua", "CLST": "Txileko udako ordua", "WEZ": "Europa mendebaldeko ordu estandarra"},
	}
}

// Locale returns the current translators string locale
func (eu *eu) Locale() string {
	return eu.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'eu'
func (eu *eu) PluralsCardinal() []locales.PluralRule {
	return eu.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'eu'
func (eu *eu) PluralsOrdinal() []locales.PluralRule {
	return eu.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'eu'
func (eu *eu) PluralsRange() []locales.PluralRule {
	return eu.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'eu'
func (eu *eu) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'eu'
func (eu *eu) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'eu'
func (eu *eu) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (eu *eu) MonthAbbreviated(month time.Month) string {
	return eu.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (eu *eu) MonthsAbbreviated() []string {
	return eu.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (eu *eu) MonthNarrow(month time.Month) string {
	return eu.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (eu *eu) MonthsNarrow() []string {
	return eu.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (eu *eu) MonthWide(month time.Month) string {
	return eu.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (eu *eu) MonthsWide() []string {
	return eu.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (eu *eu) WeekdayAbbreviated(weekday time.Weekday) string {
	return eu.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (eu *eu) WeekdaysAbbreviated() []string {
	return eu.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (eu *eu) WeekdayNarrow(weekday time.Weekday) string {
	return eu.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (eu *eu) WeekdaysNarrow() []string {
	return eu.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (eu *eu) WeekdayShort(weekday time.Weekday) string {
	return eu.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (eu *eu) WeekdaysShort() []string {
	return eu.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (eu *eu) WeekdayWide(weekday time.Weekday) string {
	return eu.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (eu *eu) WeekdaysWide() []string {
	return eu.daysWide
}

// Decimal returns the decimal point of number
func (eu *eu) Decimal() string {
	return eu.decimal
}

// Group returns the group of number
func (eu *eu) Group() string {
	return eu.group
}

// Group returns the minus sign of number
func (eu *eu) Minus() string {
	return eu.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'eu' and handles both Whole and Real numbers based on 'v'
func (eu *eu) FmtNumber(num float64, v uint64) string {

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

// FmtPercent returns 'num' with digits/precision of 'v' for 'eu' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (eu *eu) FmtPercent(num float64, v uint64) string {
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

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'eu'
func (eu *eu) FmtCurrency(num float64, v uint64, currency currency.Type) string {

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

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'eu'
// in accounting notation.
func (eu *eu) FmtAccounting(num float64, v uint64, currency currency.Type) string {

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

// FmtDateShort returns the short date representation of 't' for 'eu'
func (eu *eu) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'eu'
func (eu *eu) FmtDateMedium(t time.Time) string {

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

// FmtDateLong returns the long date representation of 't' for 'eu'
func (eu *eu) FmtDateLong(t time.Time) string {

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

// FmtDateFull returns the full date representation of 't' for 'eu'
func (eu *eu) FmtDateFull(t time.Time) string {

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

// FmtTimeShort returns the short time representation of 't' for 'eu'
func (eu *eu) FmtTimeShort(t time.Time) string {

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

// FmtTimeMedium returns the medium time representation of 't' for 'eu'
func (eu *eu) FmtTimeMedium(t time.Time) string {

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

// FmtTimeLong returns the long time representation of 't' for 'eu'
func (eu *eu) FmtTimeLong(t time.Time) string {

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

// FmtTimeFull returns the full time representation of 't' for 'eu'
func (eu *eu) FmtTimeFull(t time.Time) string {

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
