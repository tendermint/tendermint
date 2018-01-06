package mua_CM

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type mua_CM struct {
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

// New returns a new instance of translator for the 'mua_CM' locale
func New() locales.Translator {
	return &mua_CM{
		locale:                 "mua_CM",
		pluralsCardinal:        nil,
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  ".",
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: ")",
		monthsAbbreviated:      []string{"", "FLO", "CLA", "CKI", "FMF", "MAD", "MBI", "MLI", "MAM", "FDE", "FMU", "FGW", "FYU"},
		monthsNarrow:           []string{"", "O", "A", "I", "F", "D", "B", "L", "M", "E", "U", "W", "Y"},
		monthsWide:             []string{"", "Fĩi Loo", "Cokcwaklaŋne", "Cokcwaklii", "Fĩi Marfoo", "Madǝǝuutǝbijaŋ", "Mamǝŋgwãafahbii", "Mamǝŋgwãalii", "Madǝmbii", "Fĩi Dǝɓlii", "Fĩi Mundaŋ", "Fĩi Gwahlle", "Fĩi Yuru"},
		daysAbbreviated:        []string{"Cya", "Cla", "Czi", "Cko", "Cka", "Cga", "Cze"},
		daysNarrow:             []string{"Y", "L", "Z", "O", "A", "G", "E"},
		daysWide:               []string{"Com’yakke", "Comlaaɗii", "Comzyiiɗii", "Comkolle", "Comkaldǝɓlii", "Comgaisuu", "Comzyeɓsuu"},
		periodsAbbreviated:     []string{"comme", "lilli"},
		periodsWide:            []string{"comme", "lilli"},
		erasAbbreviated:        []string{"KK", "PK"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"KǝPel Kristu", "Pel Kristu"},
		timezones:              map[string]string{"WARST": "WARST", "AKDT": "AKDT", "PST": "PST", "HNCU": "HNCU", "MEZ": "MEZ", "COT": "COT", "HEEG": "HEEG", "HKST": "HKST", "CLST": "CLST", "BT": "BT", "TMT": "TMT", "LHST": "LHST", "ADT": "ADT", "CLT": "CLT", "HEPM": "HEPM", "AWST": "AWST", "WIT": "WIT", "NZST": "NZST", "HNNOMX": "HNNOMX", "ARST": "ARST", "ACDT": "ACDT", "ChST": "ChST", "HECU": "HECU", "SRT": "SRT", "HADT": "HADT", "HEPMX": "HEPMX", "WART": "WART", "ACST": "ACST", "AEDT": "AEDT", "COST": "COST", "HAST": "HAST", "OEZ": "OEZ", "HNEG": "HNEG", "EST": "EST", "MDT": "MDT", "MYT": "MYT", "TMST": "TMST", "JDT": "JDT", "GMT": "GMT", "PDT": "PDT", "CHADT": "CHADT", "MST": "MST", "HAT": "HAT", "GFT": "GFT", "ECT": "ECT", "GYT": "GYT", "CDT": "CDT", "AEST": "AEST", "ART": "ART", "WAST": "WAST", "OESZ": "OESZ", "SAST": "SAST", "EDT": "EDT", "WEZ": "WEZ", "NZDT": "NZDT", "WITA": "WITA", "JST": "JST", "WAT": "WAT", "CST": "CST", "ACWST": "ACWST", "ACWDT": "ACWDT", "VET": "VET", "UYST": "UYST", "MESZ": "MESZ", "HENOMX": "HENOMX", "HNOG": "HNOG", "WIB": "WIB", "BOT": "BOT", "AWDT": "AWDT", "UYT": "UYT", "HEOG": "HEOG", "∅∅∅": "∅∅∅", "CHAST": "CHAST", "EAT": "EAT", "AKST": "AKST", "IST": "IST", "HKT": "HKT", "CAT": "CAT", "WESZ": "WESZ", "SGT": "SGT", "HNPM": "HNPM", "HNPMX": "HNPMX", "LHDT": "LHDT", "AST": "AST", "HNT": "HNT"},
	}
}

// Locale returns the current translators string locale
func (mua *mua_CM) Locale() string {
	return mua.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'mua_CM'
func (mua *mua_CM) PluralsCardinal() []locales.PluralRule {
	return mua.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'mua_CM'
func (mua *mua_CM) PluralsOrdinal() []locales.PluralRule {
	return mua.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'mua_CM'
func (mua *mua_CM) PluralsRange() []locales.PluralRule {
	return mua.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'mua_CM'
func (mua *mua_CM) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'mua_CM'
func (mua *mua_CM) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'mua_CM'
func (mua *mua_CM) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (mua *mua_CM) MonthAbbreviated(month time.Month) string {
	return mua.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (mua *mua_CM) MonthsAbbreviated() []string {
	return mua.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (mua *mua_CM) MonthNarrow(month time.Month) string {
	return mua.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (mua *mua_CM) MonthsNarrow() []string {
	return mua.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (mua *mua_CM) MonthWide(month time.Month) string {
	return mua.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (mua *mua_CM) MonthsWide() []string {
	return mua.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (mua *mua_CM) WeekdayAbbreviated(weekday time.Weekday) string {
	return mua.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (mua *mua_CM) WeekdaysAbbreviated() []string {
	return mua.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (mua *mua_CM) WeekdayNarrow(weekday time.Weekday) string {
	return mua.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (mua *mua_CM) WeekdaysNarrow() []string {
	return mua.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (mua *mua_CM) WeekdayShort(weekday time.Weekday) string {
	return mua.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (mua *mua_CM) WeekdaysShort() []string {
	return mua.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (mua *mua_CM) WeekdayWide(weekday time.Weekday) string {
	return mua.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (mua *mua_CM) WeekdaysWide() []string {
	return mua.daysWide
}

// Decimal returns the decimal point of number
func (mua *mua_CM) Decimal() string {
	return mua.decimal
}

// Group returns the group of number
func (mua *mua_CM) Group() string {
	return mua.group
}

// Group returns the minus sign of number
func (mua *mua_CM) Minus() string {
	return mua.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'mua_CM' and handles both Whole and Real numbers based on 'v'
func (mua *mua_CM) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 1 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, mua.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, mua.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, mua.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'mua_CM' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (mua *mua_CM) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 1
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, mua.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, mua.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, mua.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'mua_CM'
func (mua *mua_CM) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := mua.currencies[currency]
	l := len(s) + len(symbol) + 1 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, mua.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, mua.group[0])
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
		b = append(b, mua.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, mua.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'mua_CM'
// in accounting notation.
func (mua *mua_CM) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := mua.currencies[currency]
	l := len(s) + len(symbol) + 3 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, mua.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, mua.group[0])
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

		b = append(b, mua.currencyNegativePrefix[0])

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
			b = append(b, mua.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, mua.currencyNegativeSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, mua.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, mua.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, mua.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, mua.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'mua_CM'
func (mua *mua_CM) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, mua.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := mua.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
