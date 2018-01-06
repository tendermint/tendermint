package ebu

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ebu struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
	perMille               string
	timeSeparator          string
	inifinity              string
	currencies             []string // idx = enum of currency code
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

// New returns a new instance of translator for the 'ebu' locale
func New() locales.Translator {
	return &ebu{
		locale:                 "ebu",
		pluralsCardinal:        nil,
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "Ksh", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: ")",
		monthsAbbreviated:      []string{"", "Mbe", "Kai", "Kat", "Kan", "Gat", "Gan", "Mug", "Knn", "Ken", "Iku", "Imw", "Igi"},
		monthsNarrow:           []string{"", "M", "K", "K", "K", "G", "G", "M", "K", "K", "I", "I", "I"},
		monthsWide:             []string{"", "Mweri wa mbere", "Mweri wa kaĩri", "Mweri wa kathatũ", "Mweri wa kana", "Mweri wa gatano", "Mweri wa gatantatũ", "Mweri wa mũgwanja", "Mweri wa kanana", "Mweri wa kenda", "Mweri wa ikũmi", "Mweri wa ikũmi na ũmwe", "Mweri wa ikũmi na Kaĩrĩ"},
		daysAbbreviated:        []string{"Kma", "Tat", "Ine", "Tan", "Arm", "Maa", "NMM"},
		daysNarrow:             []string{"K", "N", "N", "N", "A", "M", "N"},
		daysWide:               []string{"Kiumia", "Njumatatu", "Njumaine", "Njumatano", "Aramithi", "Njumaa", "NJumamothii"},
		periodsAbbreviated:     []string{"KI", "UT"},
		periodsWide:            []string{"KI", "UT"},
		erasAbbreviated:        []string{"MK", "TK"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"Mbere ya Kristo", "Thutha wa Kristo"},
		timezones:              map[string]string{"CHADT": "CHADT", "AWST": "AWST", "MYT": "MYT", "AEDT": "AEDT", "GMT": "GMT", "SGT": "SGT", "SRT": "SRT", "NZST": "NZST", "WITA": "WITA", "JST": "JST", "HEEG": "HEEG", "COST": "COST", "WIB": "WIB", "PDT": "PDT", "LHST": "LHST", "LHDT": "LHDT", "AEST": "AEST", "HNOG": "HNOG", "HKST": "HKST", "EST": "EST", "EDT": "EDT", "MESZ": "MESZ", "TMT": "TMT", "VET": "VET", "JDT": "JDT", "WARST": "WARST", "IST": "IST", "HNPM": "HNPM", "UYST": "UYST", "GYT": "GYT", "ECT": "ECT", "CHAST": "CHAST", "HECU": "HECU", "MEZ": "MEZ", "WART": "WART", "CLT": "CLT", "CLST": "CLST", "HNCU": "HNCU", "OESZ": "OESZ", "ART": "ART", "BT": "BT", "CDT": "CDT", "MDT": "MDT", "ACST": "ACST", "HNPMX": "HNPMX", "OEZ": "OEZ", "AST": "AST", "WAT": "WAT", "HNT": "HNT", "AKST": "AKST", "ACWST": "ACWST", "HAST": "HAST", "HADT": "HADT", "HENOMX": "HENOMX", "SAST": "SAST", "WAST": "WAST", "AWDT": "AWDT", "UYT": "UYT", "NZDT": "NZDT", "EAT": "EAT", "AKDT": "AKDT", "ACDT": "ACDT", "PST": "PST", "CST": "CST", "ARST": "ARST", "HNEG": "HNEG", "WESZ": "WESZ", "COT": "COT", "CAT": "CAT", "HEPM": "HEPM", "MST": "MST", "TMST": "TMST", "HNNOMX": "HNNOMX", "ADT": "ADT", "HKT": "HKT", "HEPMX": "HEPMX", "∅∅∅": "∅∅∅", "WIT": "WIT", "HEOG": "HEOG", "GFT": "GFT", "HAT": "HAT", "ChST": "ChST", "BOT": "BOT", "ACWDT": "ACWDT", "WEZ": "WEZ"},
	}
}

// Locale returns the current translators string locale
func (ebu *ebu) Locale() string {
	return ebu.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ebu'
func (ebu *ebu) PluralsCardinal() []locales.PluralRule {
	return ebu.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ebu'
func (ebu *ebu) PluralsOrdinal() []locales.PluralRule {
	return ebu.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ebu'
func (ebu *ebu) PluralsRange() []locales.PluralRule {
	return ebu.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ebu'
func (ebu *ebu) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ebu'
func (ebu *ebu) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ebu'
func (ebu *ebu) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ebu *ebu) MonthAbbreviated(month time.Month) string {
	return ebu.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ebu *ebu) MonthsAbbreviated() []string {
	return ebu.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ebu *ebu) MonthNarrow(month time.Month) string {
	return ebu.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ebu *ebu) MonthsNarrow() []string {
	return ebu.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ebu *ebu) MonthWide(month time.Month) string {
	return ebu.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ebu *ebu) MonthsWide() []string {
	return ebu.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ebu *ebu) WeekdayAbbreviated(weekday time.Weekday) string {
	return ebu.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ebu *ebu) WeekdaysAbbreviated() []string {
	return ebu.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ebu *ebu) WeekdayNarrow(weekday time.Weekday) string {
	return ebu.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ebu *ebu) WeekdaysNarrow() []string {
	return ebu.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ebu *ebu) WeekdayShort(weekday time.Weekday) string {
	return ebu.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ebu *ebu) WeekdaysShort() []string {
	return ebu.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ebu *ebu) WeekdayWide(weekday time.Weekday) string {
	return ebu.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ebu *ebu) WeekdaysWide() []string {
	return ebu.daysWide
}

// Decimal returns the decimal point of number
func (ebu *ebu) Decimal() string {
	return ebu.decimal
}

// Group returns the group of number
func (ebu *ebu) Group() string {
	return ebu.group
}

// Group returns the minus sign of number
func (ebu *ebu) Minus() string {
	return ebu.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ebu' and handles both Whole and Real numbers based on 'v'
func (ebu *ebu) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ebu' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ebu *ebu) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ebu'
func (ebu *ebu) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ebu.currencies[currency]
	l := len(s) + len(symbol) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ebu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ebu.group[0])
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
		b = append(b, ebu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ebu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ebu'
// in accounting notation.
func (ebu *ebu) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ebu.currencies[currency]
	l := len(s) + len(symbol) + 2
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ebu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ebu.group[0])
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

		b = append(b, ebu.currencyNegativePrefix[0])

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
			b = append(b, ebu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, ebu.currencyNegativeSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ebu'
func (ebu *ebu) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'ebu'
func (ebu *ebu) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ebu.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ebu'
func (ebu *ebu) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ebu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ebu'
func (ebu *ebu) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ebu.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ebu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ebu'
func (ebu *ebu) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ebu'
func (ebu *ebu) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ebu'
func (ebu *ebu) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ebu'
func (ebu *ebu) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ebu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ebu.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
