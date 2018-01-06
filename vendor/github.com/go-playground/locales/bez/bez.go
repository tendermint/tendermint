package bez

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type bez struct {
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

// New returns a new instance of translator for the 'bez' locale
func New() locales.Translator {
	return &bez{
		locale:             "bez",
		pluralsCardinal:    []locales.PluralRule{2, 6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		timeSeparator:      ":",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TSh", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "Hut", "Vil", "Dat", "Tai", "Han", "Sit", "Sab", "Nan", "Tis", "Kum", "Kmj", "Kmb"},
		monthsNarrow:       []string{"", "H", "V", "D", "T", "H", "S", "S", "N", "T", "K", "K", "K"},
		monthsWide:         []string{"", "pa mwedzi gwa hutala", "pa mwedzi gwa wuvili", "pa mwedzi gwa wudatu", "pa mwedzi gwa wutai", "pa mwedzi gwa wuhanu", "pa mwedzi gwa sita", "pa mwedzi gwa saba", "pa mwedzi gwa nane", "pa mwedzi gwa tisa", "pa mwedzi gwa kumi", "pa mwedzi gwa kumi na moja", "pa mwedzi gwa kumi na mbili"},
		daysAbbreviated:    []string{"Mul", "Vil", "Hiv", "Hid", "Hit", "Hih", "Lem"},
		daysNarrow:         []string{"M", "J", "H", "H", "H", "W", "J"},
		daysWide:           []string{"pa mulungu", "pa shahuviluha", "pa hivili", "pa hidatu", "pa hitayi", "pa hihanu", "pa shahulembela"},
		periodsAbbreviated: []string{"pamilau", "pamunyi"},
		periodsWide:        []string{"pamilau", "pamunyi"},
		erasAbbreviated:    []string{"KM", "BM"},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"Kabla ya Mtwaa", "Baada ya Mtwaa"},
		timezones:          map[string]string{"ChST": "ChST", "HNPM": "HNPM", "ACWDT": "ACWDT", "TMST": "TMST", "HKT": "HKT", "COT": "COT", "COST": "COST", "AKDT": "AKDT", "IST": "IST", "HNOG": "HNOG", "HEOG": "HEOG", "WAST": "WAST", "EST": "EST", "ACST": "ACST", "ECT": "ECT", "AWST": "AWST", "CLT": "CLT", "MDT": "MDT", "NZST": "NZST", "ARST": "ARST", "HKST": "HKST", "WIB": "WIB", "AWDT": "AWDT", "ACWST": "ACWST", "PST": "PST", "LHST": "LHST", "AST": "AST", "HEEG": "HEEG", "GYT": "GYT", "HEPMX": "HEPMX", "BT": "BT", "JDT": "JDT", "ACDT": "ACDT", "HEPM": "HEPM", "HADT": "HADT", "LHDT": "LHDT", "OESZ": "OESZ", "WARST": "WARST", "ART": "ART", "EDT": "EDT", "CHADT": "CHADT", "SRT": "SRT", "WITA": "WITA", "JST": "JST", "AEDT": "AEDT", "EAT": "EAT", "∅∅∅": "∅∅∅", "CAT": "CAT", "MESZ": "MESZ", "HAST": "HAST", "CHAST": "CHAST", "WIT": "WIT", "HECU": "HECU", "CDT": "CDT", "HNNOMX": "HNNOMX", "AEST": "AEST", "HNT": "HNT", "AKST": "AKST", "WESZ": "WESZ", "PDT": "PDT", "HNEG": "HNEG", "CLST": "CLST", "BOT": "BOT", "MST": "MST", "WART": "WART", "CST": "CST", "UYST": "UYST", "MEZ": "MEZ", "NZDT": "NZDT", "HAT": "HAT", "WEZ": "WEZ", "GMT": "GMT", "HNPMX": "HNPMX", "HENOMX": "HENOMX", "SAST": "SAST", "GFT": "GFT", "SGT": "SGT", "HNCU": "HNCU", "VET": "VET", "TMT": "TMT", "OEZ": "OEZ", "MYT": "MYT", "UYT": "UYT", "ADT": "ADT", "WAT": "WAT"},
	}
}

// Locale returns the current translators string locale
func (bez *bez) Locale() string {
	return bez.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'bez'
func (bez *bez) PluralsCardinal() []locales.PluralRule {
	return bez.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'bez'
func (bez *bez) PluralsOrdinal() []locales.PluralRule {
	return bez.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'bez'
func (bez *bez) PluralsRange() []locales.PluralRule {
	return bez.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'bez'
func (bez *bez) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'bez'
func (bez *bez) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'bez'
func (bez *bez) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (bez *bez) MonthAbbreviated(month time.Month) string {
	return bez.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (bez *bez) MonthsAbbreviated() []string {
	return bez.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (bez *bez) MonthNarrow(month time.Month) string {
	return bez.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (bez *bez) MonthsNarrow() []string {
	return bez.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (bez *bez) MonthWide(month time.Month) string {
	return bez.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (bez *bez) MonthsWide() []string {
	return bez.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (bez *bez) WeekdayAbbreviated(weekday time.Weekday) string {
	return bez.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (bez *bez) WeekdaysAbbreviated() []string {
	return bez.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (bez *bez) WeekdayNarrow(weekday time.Weekday) string {
	return bez.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (bez *bez) WeekdaysNarrow() []string {
	return bez.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (bez *bez) WeekdayShort(weekday time.Weekday) string {
	return bez.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (bez *bez) WeekdaysShort() []string {
	return bez.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (bez *bez) WeekdayWide(weekday time.Weekday) string {
	return bez.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (bez *bez) WeekdaysWide() []string {
	return bez.daysWide
}

// Decimal returns the decimal point of number
func (bez *bez) Decimal() string {
	return bez.decimal
}

// Group returns the group of number
func (bez *bez) Group() string {
	return bez.group
}

// Group returns the minus sign of number
func (bez *bez) Minus() string {
	return bez.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'bez' and handles both Whole and Real numbers based on 'v'
func (bez *bez) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'bez' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (bez *bez) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'bez'
func (bez *bez) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := bez.currencies[currency]
	l := len(s) + len(symbol) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, bez.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, bez.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, bez.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, bez.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'bez'
// in accounting notation.
func (bez *bez) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := bez.currencies[currency]
	l := len(s) + len(symbol) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, bez.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, bez.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, bez.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, bez.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, symbol...)
	} else {

		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'bez'
func (bez *bez) FmtDateShort(t time.Time) string {

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

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'bez'
func (bez *bez) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, bez.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'bez'
func (bez *bez) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, bez.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'bez'
func (bez *bez) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, bez.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, bez.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'bez'
func (bez *bez) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'bez'
func (bez *bez) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'bez'
func (bez *bez) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'bez'
func (bez *bez) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, bez.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := bez.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
