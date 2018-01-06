package kl_GL

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type kl_GL struct {
	locale             string
	pluralsCardinal    []locales.PluralRule
	pluralsOrdinal     []locales.PluralRule
	pluralsRange       []locales.PluralRule
	decimal            string
	group              string
	minus              string
	percent            string
	percentSuffix      string
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

// New returns a new instance of translator for the 'kl_GL' locale
func New() locales.Translator {
	return &kl_GL{
		locale:             "kl_GL",
		pluralsCardinal:    []locales.PluralRule{2, 6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		decimal:            ",",
		group:              ".",
		minus:              "−",
		percent:            "%",
		perMille:           "‰",
		timeSeparator:      ":",
		inifinity:          "∞",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:      " ",
		monthsAbbreviated:  []string{"", "jan", "feb", "mar", "apr", "maj", "jun", "jul", "aug", "sep", "okt", "nov", "dec"},
		monthsNarrow:       []string{"", "J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"},
		monthsWide:         []string{"", "januari", "februari", "martsi", "aprili", "maji", "juni", "juli", "augustusi", "septemberi", "oktoberi", "novemberi", "decemberi"},
		daysAbbreviated:    []string{"sab", "ata", "mar", "pin", "sis", "tal", "arf"},
		daysNarrow:         []string{"S", "A", "M", "P", "S", "T", "A"},
		daysShort:          []string{"sab", "ata", "mar", "pin", "sis", "tal", "arf"},
		daysWide:           []string{"sabaat", "ataasinngorneq", "marlunngorneq", "pingasunngorneq", "sisamanngorneq", "tallimanngorneq", "arfininngorneq"},
		periodsAbbreviated: []string{"u.t.", "u.k."},
		periodsWide:        []string{"ulloqeqqata-tungaa", "ulloqeqqata-kingorna"},
		erasAbbreviated:    []string{"Kr.in.si.", "Kr.in.king."},
		erasNarrow:         []string{"Kr.s.", "Kr.k."},
		erasWide:           []string{"Kristusip inunngornerata siornagut", "Kristusip inunngornerata kingornagut"},
		timezones:          map[string]string{"VET": "VET", "HEOG": "HEOG", "GFT": "GFT", "CLST": "CLST", "EST": "EST", "GMT": "GMT", "CHADT": "CHADT", "ACWST": "ACWST", "HNCU": "HNCU", "ART": "ART", "ARST": "ARST", "HNOG": "HNOG", "HNEG": "HNEG", "ECT": "ECT", "OESZ": "OESZ", "MESZ": "MESZ", "HENOMX": "HENOMX", "WITA": "WITA", "JDT": "JDT", "LHST": "LHST", "LHDT": "LHDT", "COST": "COST", "AWST": "AWST", "EAT": "EAT", "HNT": "HNT", "HKST": "HKST", "WIT": "WIT", "NZST": "NZST", "WART": "WART", "WARST": "WARST", "AEST": "AEST", "WAT": "WAT", "HKT": "HKT", "WEZ": "WEZ", "MEZ": "MEZ", "HNPM": "HNPM", "AKST": "AKST", "CDT": "CDT", "HADT": "HADT", "HAT": "HAT", "ACDT": "ACDT", "CAT": "CAT", "CHAST": "CHAST", "ACWDT": "ACWDT", "COT": "COT", "HEPMX": "HEPMX", "IST": "IST", "MDT": "MDT", "AST": "AST", "ChST": "ChST", "BOT": "BOT", "MST": "MST", "UYT": "UYT", "TMT": "TMT", "AEDT": "AEDT", "AKDT": "AKDT", "SGT": "SGT", "BT": "BT", "JST": "JST", "CLT": "CLT", "WIB": "WIB", "MYT": "MYT", "UYST": "UYST", "HNNOMX": "HNNOMX", "WAST": "WAST", "HNPMX": "HNPMX", "HEPM": "HEPM", "SRT": "SRT", "GYT": "GYT", "SAST": "SAST", "TMST": "TMST", "OEZ": "OEZ", "ADT": "ADT", "EDT": "EDT", "ACST": "ACST", "WESZ": "WESZ", "CST": "CST", "HAST": "HAST", "PST": "PST", "PDT": "PDT", "HECU": "HECU", "∅∅∅": "∅∅∅", "NZDT": "NZDT", "HEEG": "HEEG", "AWDT": "AWDT"},
	}
}

// Locale returns the current translators string locale
func (kl *kl_GL) Locale() string {
	return kl.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'kl_GL'
func (kl *kl_GL) PluralsCardinal() []locales.PluralRule {
	return kl.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'kl_GL'
func (kl *kl_GL) PluralsOrdinal() []locales.PluralRule {
	return kl.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'kl_GL'
func (kl *kl_GL) PluralsRange() []locales.PluralRule {
	return kl.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'kl_GL'
func (kl *kl_GL) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'kl_GL'
func (kl *kl_GL) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'kl_GL'
func (kl *kl_GL) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (kl *kl_GL) MonthAbbreviated(month time.Month) string {
	return kl.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (kl *kl_GL) MonthsAbbreviated() []string {
	return kl.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (kl *kl_GL) MonthNarrow(month time.Month) string {
	return kl.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (kl *kl_GL) MonthsNarrow() []string {
	return kl.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (kl *kl_GL) MonthWide(month time.Month) string {
	return kl.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (kl *kl_GL) MonthsWide() []string {
	return kl.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (kl *kl_GL) WeekdayAbbreviated(weekday time.Weekday) string {
	return kl.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (kl *kl_GL) WeekdaysAbbreviated() []string {
	return kl.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (kl *kl_GL) WeekdayNarrow(weekday time.Weekday) string {
	return kl.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (kl *kl_GL) WeekdaysNarrow() []string {
	return kl.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (kl *kl_GL) WeekdayShort(weekday time.Weekday) string {
	return kl.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (kl *kl_GL) WeekdaysShort() []string {
	return kl.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (kl *kl_GL) WeekdayWide(weekday time.Weekday) string {
	return kl.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (kl *kl_GL) WeekdaysWide() []string {
	return kl.daysWide
}

// Decimal returns the decimal point of number
func (kl *kl_GL) Decimal() string {
	return kl.decimal
}

// Group returns the group of number
func (kl *kl_GL) Group() string {
	return kl.group
}

// Group returns the minus sign of number
func (kl *kl_GL) Minus() string {
	return kl.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'kl_GL' and handles both Whole and Real numbers based on 'v'
func (kl *kl_GL) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kl.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, kl.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(kl.minus) - 1; j >= 0; j-- {
			b = append(b, kl.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'kl_GL' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (kl *kl_GL) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 7
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kl.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(kl.minus) - 1; j >= 0; j-- {
			b = append(b, kl.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, kl.percentSuffix...)

	b = append(b, kl.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'kl_GL'
func (kl *kl_GL) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kl.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kl.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, kl.group[0])
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
		for j := len(kl.minus) - 1; j >= 0; j-- {
			b = append(b, kl.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, kl.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'kl_GL'
// in accounting notation.
func (kl *kl_GL) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kl.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kl.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, kl.group[0])
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

		for j := len(kl.minus) - 1; j >= 0; j-- {
			b = append(b, kl.minus[j])
		}

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
			b = append(b, kl.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x2d}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2d}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, kl.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kl.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, kl.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kl.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kl.periodsAbbreviated[0]...)
	} else {
		b = append(b, kl.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kl.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kl.periodsAbbreviated[0]...)
	} else {
		b = append(b, kl.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kl.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kl.periodsAbbreviated[0]...)
	} else {
		b = append(b, kl.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'kl_GL'
func (kl *kl_GL) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kl.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kl.periodsAbbreviated[0]...)
	} else {
		b = append(b, kl.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := kl.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
