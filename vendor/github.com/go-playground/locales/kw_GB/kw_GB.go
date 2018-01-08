package kw_GB

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type kw_GB struct {
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

// New returns a new instance of translator for the 'kw_GB' locale
func New() locales.Translator {
	return &kw_GB{
		locale:             "kw_GB",
		pluralsCardinal:    []locales.PluralRule{2, 3, 6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		timeSeparator:      ":",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "Gen", "Hwe", "Meu", "Ebr", "Me", "Met", "Gor", "Est", "Gwn", "Hed", "Du", "Kev"},
		monthsWide:         []string{"", "mis Genver", "mis Hwevrer", "mis Meurth", "mis Ebrel", "mis Me", "mis Metheven", "mis Gortheren", "mis Est", "mis Gwynngala", "mis Hedra", "mis Du", "mis Kevardhu"},
		daysAbbreviated:    []string{"Sul", "Lun", "Mth", "Mhr", "Yow", "Gwe", "Sad"},
		daysWide:           []string{"dy Sul", "dy Lun", "dy Meurth", "dy Merher", "dy Yow", "dy Gwener", "dy Sadorn"},
		periodsAbbreviated: []string{"a.m.", "p.m."},
		periodsWide:        []string{"a.m.", "p.m."},
		erasAbbreviated:    []string{"RC", "AD"},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"", ""},
		timezones:          map[string]string{"IST": "IST", "ADT": "ADT", "AEST": "AEST", "HNOG": "HNOG", "CLST": "CLST", "HNCU": "HNCU", "HNPM": "HNPM", "MEZ": "MEZ", "CHAST": "CHAST", "HENOMX": "HENOMX", "SAST": "SAST", "HNT": "HNT", "HAT": "HAT", "HNPMX": "HNPMX", "CLT": "CLT", "GFT": "GFT", "JDT": "JDT", "MST": "MST", "HAST": "HAST", "JST": "JST", "PDT": "PDT", "MYT": "MYT", "OEZ": "OEZ", "CST": "CST", "CDT": "CDT", "NZST": "NZST", "TMT": "TMT", "WAST": "WAST", "ACST": "ACST", "WESZ": "WESZ", "PST": "PST", "OESZ": "OESZ", "ACWST": "ACWST", "LHST": "LHST", "HEOG": "HEOG", "HKST": "HKST", "GMT": "GMT", "HEPMX": "HEPMX", "MDT": "MDT", "WITA": "WITA", "HNEG": "HNEG", "HKT": "HKT", "COST": "COST", "ACDT": "ACDT", "EDT": "EDT", "WIB": "WIB", "HECU": "HECU", "WARST": "WARST", "EAT": "EAT", "EST": "EST", "AWDT": "AWDT", "LHDT": "LHDT", "AWST": "AWST", "AEDT": "AEDT", "GYT": "GYT", "AKDT": "AKDT", "WEZ": "WEZ", "BOT": "BOT", "AKST": "AKST", "SGT": "SGT", "UYT": "UYT", "WIT": "WIT", "UYST": "UYST", "HADT": "HADT", "COT": "COT", "CAT": "CAT", "ChST": "ChST", "HEPM": "HEPM", "∅∅∅": "∅∅∅", "CHADT": "CHADT", "MESZ": "MESZ", "VET": "VET", "HNNOMX": "HNNOMX", "AST": "AST", "ARST": "ARST", "HEEG": "HEEG", "ECT": "ECT", "NZDT": "NZDT", "TMST": "TMST", "WART": "WART", "ART": "ART", "WAT": "WAT", "BT": "BT", "SRT": "SRT", "ACWDT": "ACWDT"},
	}
}

// Locale returns the current translators string locale
func (kw *kw_GB) Locale() string {
	return kw.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'kw_GB'
func (kw *kw_GB) PluralsCardinal() []locales.PluralRule {
	return kw.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'kw_GB'
func (kw *kw_GB) PluralsOrdinal() []locales.PluralRule {
	return kw.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'kw_GB'
func (kw *kw_GB) PluralsRange() []locales.PluralRule {
	return kw.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'kw_GB'
func (kw *kw_GB) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	} else if n == 2 {
		return locales.PluralRuleTwo
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'kw_GB'
func (kw *kw_GB) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'kw_GB'
func (kw *kw_GB) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (kw *kw_GB) MonthAbbreviated(month time.Month) string {
	return kw.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (kw *kw_GB) MonthsAbbreviated() []string {
	return kw.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (kw *kw_GB) MonthNarrow(month time.Month) string {
	return kw.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (kw *kw_GB) MonthsNarrow() []string {
	return nil
}

// MonthWide returns the locales wide month given the 'month' provided
func (kw *kw_GB) MonthWide(month time.Month) string {
	return kw.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (kw *kw_GB) MonthsWide() []string {
	return kw.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (kw *kw_GB) WeekdayAbbreviated(weekday time.Weekday) string {
	return kw.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (kw *kw_GB) WeekdaysAbbreviated() []string {
	return kw.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (kw *kw_GB) WeekdayNarrow(weekday time.Weekday) string {
	return kw.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (kw *kw_GB) WeekdaysNarrow() []string {
	return kw.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (kw *kw_GB) WeekdayShort(weekday time.Weekday) string {
	return kw.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (kw *kw_GB) WeekdaysShort() []string {
	return kw.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (kw *kw_GB) WeekdayWide(weekday time.Weekday) string {
	return kw.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (kw *kw_GB) WeekdaysWide() []string {
	return kw.daysWide
}

// Decimal returns the decimal point of number
func (kw *kw_GB) Decimal() string {
	return kw.decimal
}

// Group returns the group of number
func (kw *kw_GB) Group() string {
	return kw.group
}

// Group returns the minus sign of number
func (kw *kw_GB) Minus() string {
	return kw.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'kw_GB' and handles both Whole and Real numbers based on 'v'
func (kw *kw_GB) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'kw_GB' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (kw *kw_GB) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'kw_GB'
func (kw *kw_GB) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kw.currencies[currency]
	l := len(s) + len(symbol) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kw.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, kw.group[0])
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
		b = append(b, kw.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, kw.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'kw_GB'
// in accounting notation.
func (kw *kw_GB) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kw.currencies[currency]
	l := len(s) + len(symbol) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kw.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, kw.group[0])
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

		b = append(b, kw.minus[0])

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
			b = append(b, kw.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kw.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kw.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, kw.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kw.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'kw_GB'
func (kw *kw_GB) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kw.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := kw.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
