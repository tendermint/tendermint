package ti

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ti struct {
	locale             string
	pluralsCardinal    []locales.PluralRule
	pluralsOrdinal     []locales.PluralRule
	pluralsRange       []locales.PluralRule
	decimal            string
	group              string
	minus              string
	percent            string
	perMille           string
	timeSeparator      string
	inifinity          string
	currencies         []string // idx = enum of currency code
	monthsAbbreviated  []string
	monthsNarrow       []string
	monthsWide         []string
	daysAbbreviated    []string
	daysNarrow         []string
	daysShort          []string
	daysWide           []string
	periodsAbbreviated []string
	periodsNarrow      []string
	periodsShort       []string
	periodsWide        []string
	erasAbbreviated    []string
	erasNarrow         []string
	erasWide           []string
	timezones          map[string]string
}

// New returns a new instance of translator for the 'ti' locale
func New() locales.Translator {
	return &ti{
		locale:             "ti",
		pluralsCardinal:    []locales.PluralRule{2, 6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		decimal:            ".",
		group:              ",",
		minus:              "-",
		percent:            "%",
		perMille:           "‰",
		timeSeparator:      ":",
		inifinity:          "∞",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "Br", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "ጥሪ", "ለካ", "መጋ", "ሚያ", "ግን", "ሰነ", "ሓም", "ነሓ", "መስ", "ጥቅ", "ሕዳ", "ታሕ"},
		monthsNarrow:       []string{"", "ጥ", "ለ", "መ", "ሚ", "ግ", "ሰ", "ሓ", "ነ", "መ", "ጥ", "ሕ", "ታ"},
		monthsWide:         []string{"", "ጥሪ", "ለካቲት", "መጋቢት", "ሚያዝያ", "ግንቦት", "ሰነ", "ሓምለ", "ነሓሰ", "መስከረም", "ጥቅምቲ", "ሕዳር", "ታሕሳስ"},
		daysAbbreviated:    []string{"ሰን", "ሰኑ", "ሰሉ", "ረቡ", "ሓሙ", "ዓር", "ቀዳ"},
		daysNarrow:         []string{"ሰ", "ሰ", "ሰ", "ረ", "ሓ", "ዓ", "ቀ"},
		daysShort:          []string{"ሰን", "ሰኑ", "ሰሉ", "ረቡ", "ሓሙ", "ዓር", "ቀዳ"},
		daysWide:           []string{"ሰንበት", "ሰኑይ", "ሠሉስ", "ረቡዕ", "ኃሙስ", "ዓርቢ", "ቀዳም"},
		periodsAbbreviated: []string{"ንጉሆ ሰዓተ", "ድሕር ሰዓት"},
		periodsWide:        []string{"ንጉሆ ሰዓተ", "ድሕር ሰዓት"},
		erasAbbreviated:    []string{"ዓ/ዓ", "ዓ/ም"},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"ዓ/ዓ", "ዓመተ ምህረት"},
		timezones:          map[string]string{"GYT": "GYT", "WIB": "WIB", "SRT": "SRT", "HNNOMX": "HNNOMX", "HNPMX": "HNPMX", "AWDT": "AWDT", "AEST": "AEST", "SAST": "SAST", "HEEG": "HEEG", "HAT": "HAT", "HKT": "HKT", "SGT": "SGT", "WIT": "WIT", "MEZ": "MEZ", "COT": "COT", "HNCU": "HNCU", "AWST": "AWST", "MYT": "MYT", "JST": "JST", "AST": "AST", "GFT": "GFT", "ECT": "ECT", "CDT": "CDT", "LHST": "LHST", "WITA": "WITA", "∅∅∅": "∅∅∅", "CAT": "CAT", "PDT": "PDT", "HEPM": "HEPM", "AEDT": "AEDT", "HNPM": "HNPM", "BT": "BT", "MST": "MST", "HKST": "HKST", "WEZ": "WEZ", "OESZ": "OESZ", "HNEG": "HNEG", "ACDT": "ACDT", "PST": "PST", "HAST": "HAST", "WART": "WART", "CLT": "CLT", "EDT": "EDT", "AKST": "AKST", "WESZ": "WESZ", "CHAST": "CHAST", "CHADT": "CHADT", "ADT": "ADT", "ACST": "ACST", "BOT": "BOT", "TMT": "TMT", "TMST": "TMST", "EAT": "EAT", "CST": "CST", "MDT": "MDT", "ACWST": "ACWST", "MESZ": "MESZ", "JDT": "JDT", "ACWDT": "ACWDT", "UYT": "UYT", "ART": "ART", "ARST": "ARST", "HNOG": "HNOG", "CLST": "CLST", "EST": "EST", "HEPMX": "HEPMX", "NZDT": "NZDT", "VET": "VET", "HENOMX": "HENOMX", "HEOG": "HEOG", "WAST": "WAST", "COST": "COST", "GMT": "GMT", "ChST": "ChST", "IST": "IST", "AKDT": "AKDT", "HECU": "HECU", "HADT": "HADT", "NZST": "NZST", "WARST": "WARST", "WAT": "WAT", "HNT": "HNT", "UYST": "UYST", "OEZ": "OEZ", "LHDT": "LHDT"},
	}
}

// Locale returns the current translators string locale
func (ti *ti) Locale() string {
	return ti.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ti'
func (ti *ti) PluralsCardinal() []locales.PluralRule {
	return ti.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ti'
func (ti *ti) PluralsOrdinal() []locales.PluralRule {
	return ti.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ti'
func (ti *ti) PluralsRange() []locales.PluralRule {
	return ti.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ti'
func (ti *ti) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n >= 0 && n <= 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ti'
func (ti *ti) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ti'
func (ti *ti) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ti *ti) MonthAbbreviated(month time.Month) string {
	return ti.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ti *ti) MonthsAbbreviated() []string {
	return ti.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ti *ti) MonthNarrow(month time.Month) string {
	return ti.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ti *ti) MonthsNarrow() []string {
	return ti.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ti *ti) MonthWide(month time.Month) string {
	return ti.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ti *ti) MonthsWide() []string {
	return ti.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ti *ti) WeekdayAbbreviated(weekday time.Weekday) string {
	return ti.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ti *ti) WeekdaysAbbreviated() []string {
	return ti.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ti *ti) WeekdayNarrow(weekday time.Weekday) string {
	return ti.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ti *ti) WeekdaysNarrow() []string {
	return ti.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ti *ti) WeekdayShort(weekday time.Weekday) string {
	return ti.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ti *ti) WeekdaysShort() []string {
	return ti.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ti *ti) WeekdayWide(weekday time.Weekday) string {
	return ti.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ti *ti) WeekdaysWide() []string {
	return ti.daysWide
}

// Decimal returns the decimal point of number
func (ti *ti) Decimal() string {
	return ti.decimal
}

// Group returns the group of number
func (ti *ti) Group() string {
	return ti.group
}

// Group returns the minus sign of number
func (ti *ti) Minus() string {
	return ti.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ti' and handles both Whole and Real numbers based on 'v'
func (ti *ti) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ti.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ti.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ti.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ti' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ti *ti) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ti.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ti.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, ti.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ti'
func (ti *ti) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ti.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ti.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ti.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	if num < 0 {
		b = append(b, ti.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ti.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ti'
// in accounting notation.
func (ti *ti) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ti.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ti.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ti.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		b = append(b, ti.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ti.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ti'
func (ti *ti) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'ti'
func (ti *ti) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2d}...)
	b = append(b, ti.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2d}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ti'
func (ti *ti) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ti.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ti'
func (ti *ti) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ti.daysWide[t.Weekday()]...)
	b = append(b, []byte{0xe1, 0x8d, 0xa3, 0x20}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ti.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0xe1, 0x88, 0x98, 0xe1, 0x8b, 0x93, 0xe1, 0x88, 0x8d, 0xe1, 0x89, 0xb2, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)

	if t.Year() < 0 {
		b = append(b, ti.erasWide[0]...)
	} else {
		b = append(b, ti.erasWide[1]...)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ti'
func (ti *ti) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ti.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ti.periodsAbbreviated[0]...)
	} else {
		b = append(b, ti.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ti'
func (ti *ti) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ti.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ti.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ti.periodsAbbreviated[0]...)
	} else {
		b = append(b, ti.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ti'
func (ti *ti) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ti.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ti.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ti.periodsAbbreviated[0]...)
	} else {
		b = append(b, ti.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ti'
func (ti *ti) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ti.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ti.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ti.periodsAbbreviated[0]...)
	} else {
		b = append(b, ti.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ti.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
