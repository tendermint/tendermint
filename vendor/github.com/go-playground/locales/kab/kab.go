package kab

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type kab struct {
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

// New returns a new instance of translator for the 'kab' locale
func New() locales.Translator {
	return &kab{
		locale:             "kab",
		pluralsCardinal:    []locales.PluralRule{2, 6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		decimal:            ",",
		group:              " ",
		timeSeparator:      ":",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DA", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "Yen", "Fur", "Meɣ", "Yeb", "May", "Yun", "Yul", "Ɣuc", "Cte", "Tub", "Nun", "Duǧ"},
		monthsNarrow:       []string{"", "Y", "F", "M", "Y", "M", "Y", "Y", "Ɣ", "C", "T", "N", "D"},
		monthsWide:         []string{"", "Yennayer", "Fuṛar", "Meɣres", "Yebrir", "Mayyu", "Yunyu", "Yulyu", "Ɣuct", "Ctembeṛ", "Tubeṛ", "Nunembeṛ", "Duǧembeṛ"},
		daysAbbreviated:    []string{"Yan", "San", "Kraḍ", "Kuẓ", "Sam", "Sḍis", "Say"},
		daysNarrow:         []string{"Y", "S", "K", "K", "S", "S", "S"},
		daysWide:           []string{"Yanass", "Sanass", "Kraḍass", "Kuẓass", "Samass", "Sḍisass", "Sayass"},
		periodsAbbreviated: []string{"n tufat", "n tmeddit"},
		periodsWide:        []string{"n tufat", "n tmeddit"},
		erasAbbreviated:    []string{"snd. T.Ɛ", "sld. T.Ɛ"},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"send talalit n Ɛisa", "seld talalit n Ɛisa"},
		timezones:          map[string]string{"WARST": "WARST", "ADT": "ADT", "CHAST": "CHAST", "SRT": "SRT", "WART": "WART", "HNT": "HNT", "BOT": "BOT", "MDT": "MDT", "HNEG": "HNEG", "ART": "ART", "HAT": "HAT", "WEZ": "WEZ", "HNCU": "HNCU", "CDT": "CDT", "AEST": "AEST", "PST": "PST", "COST": "COST", "TMT": "TMT", "OESZ": "OESZ", "EAT": "EAT", "AKST": "AKST", "ChST": "ChST", "HNPMX": "HNPMX", "UYST": "UYST", "WITA": "WITA", "ARST": "ARST", "CLST": "CLST", "UYT": "UYT", "VET": "VET", "HNNOMX": "HNNOMX", "ACDT": "ACDT", "∅∅∅": "∅∅∅", "SAST": "SAST", "GYT": "GYT", "CST": "CST", "LHST": "LHST", "AST": "AST", "HKT": "HKT", "EST": "EST", "WIB": "WIB", "CHADT": "CHADT", "AWST": "AWST", "CAT": "CAT", "EDT": "EDT", "SGT": "SGT", "HEPM": "HEPM", "PDT": "PDT", "MESZ": "MESZ", "HECU": "HECU", "HKST": "HKST", "ECT": "ECT", "OEZ": "OEZ", "HEOG": "HEOG", "HEEG": "HEEG", "ACWST": "ACWST", "NZST": "NZST", "JDT": "JDT", "MEZ": "MEZ", "WAT": "WAT", "AKDT": "AKDT", "GMT": "GMT", "HNPM": "HNPM", "ACST": "ACST", "HNOG": "HNOG", "COT": "COT", "HEPMX": "HEPMX", "MST": "MST", "NZDT": "NZDT", "LHDT": "LHDT", "JST": "JST", "WAST": "WAST", "CLT": "CLT", "GFT": "GFT", "MYT": "MYT", "HADT": "HADT", "TMST": "TMST", "ACWDT": "ACWDT", "HAST": "HAST", "HENOMX": "HENOMX", "IST": "IST", "AEDT": "AEDT", "BT": "BT", "AWDT": "AWDT", "WIT": "WIT", "WESZ": "WESZ"},
	}
}

// Locale returns the current translators string locale
func (kab *kab) Locale() string {
	return kab.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'kab'
func (kab *kab) PluralsCardinal() []locales.PluralRule {
	return kab.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'kab'
func (kab *kab) PluralsOrdinal() []locales.PluralRule {
	return kab.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'kab'
func (kab *kab) PluralsRange() []locales.PluralRule {
	return kab.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'kab'
func (kab *kab) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 0 || i == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'kab'
func (kab *kab) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'kab'
func (kab *kab) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (kab *kab) MonthAbbreviated(month time.Month) string {
	return kab.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (kab *kab) MonthsAbbreviated() []string {
	return kab.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (kab *kab) MonthNarrow(month time.Month) string {
	return kab.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (kab *kab) MonthsNarrow() []string {
	return kab.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (kab *kab) MonthWide(month time.Month) string {
	return kab.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (kab *kab) MonthsWide() []string {
	return kab.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (kab *kab) WeekdayAbbreviated(weekday time.Weekday) string {
	return kab.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (kab *kab) WeekdaysAbbreviated() []string {
	return kab.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (kab *kab) WeekdayNarrow(weekday time.Weekday) string {
	return kab.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (kab *kab) WeekdaysNarrow() []string {
	return kab.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (kab *kab) WeekdayShort(weekday time.Weekday) string {
	return kab.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (kab *kab) WeekdaysShort() []string {
	return kab.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (kab *kab) WeekdayWide(weekday time.Weekday) string {
	return kab.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (kab *kab) WeekdaysWide() []string {
	return kab.daysWide
}

// Decimal returns the decimal point of number
func (kab *kab) Decimal() string {
	return kab.decimal
}

// Group returns the group of number
func (kab *kab) Group() string {
	return kab.group
}

// Group returns the minus sign of number
func (kab *kab) Minus() string {
	return kab.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'kab' and handles both Whole and Real numbers based on 'v'
func (kab *kab) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'kab' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (kab *kab) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'kab'
func (kab *kab) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kab.currencies[currency]
	l := len(s) + len(symbol) + 1 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kab.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(kab.group) - 1; j >= 0; j-- {
					b = append(b, kab.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, kab.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, kab.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'kab'
// in accounting notation.
func (kab *kab) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kab.currencies[currency]
	l := len(s) + len(symbol) + 1 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kab.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(kab.group) - 1; j >= 0; j-- {
					b = append(b, kab.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, kab.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, kab.decimal...)
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

// FmtDateShort returns the short date representation of 't' for 'kab'
func (kab *kab) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'kab'
func (kab *kab) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kab.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'kab'
func (kab *kab) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kab.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'kab'
func (kab *kab) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, kab.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kab.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'kab'
func (kab *kab) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kab.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kab.periodsAbbreviated[0]...)
	} else {
		b = append(b, kab.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'kab'
func (kab *kab) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kab.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kab.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kab.periodsAbbreviated[0]...)
	} else {
		b = append(b, kab.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'kab'
func (kab *kab) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kab.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kab.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kab.periodsAbbreviated[0]...)
	} else {
		b = append(b, kab.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'kab'
func (kab *kab) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, kab.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kab.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, kab.periodsAbbreviated[0]...)
	} else {
		b = append(b, kab.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := kab.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
