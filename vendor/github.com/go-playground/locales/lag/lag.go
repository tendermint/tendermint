package lag

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type lag struct {
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
	currencyPositivePrefix string
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

// New returns a new instance of translator for the 'lag' locale
func New() locales.Translator {
	return &lag{
		locale:                 "lag",
		pluralsCardinal:        []locales.PluralRule{1, 2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TSh", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositivePrefix: " ",
		currencyPositiveSuffix: "K",
		currencyNegativePrefix: " ",
		currencyNegativeSuffix: "K",
		monthsAbbreviated:      []string{"", "Fúngatɨ", "Naanɨ", "Keenda", "Ikúmi", "Inyambala", "Idwaata", "Mʉʉnchɨ", "Vɨɨrɨ", "Saatʉ", "Inyi", "Saano", "Sasatʉ"},
		monthsNarrow:           []string{"", "F", "N", "K", "I", "I", "I", "M", "V", "S", "I", "S", "S"},
		monthsWide:             []string{"", "Kʉfúngatɨ", "Kʉnaanɨ", "Kʉkeenda", "Kwiikumi", "Kwiinyambála", "Kwiidwaata", "Kʉmʉʉnchɨ", "Kʉvɨɨrɨ", "Kʉsaatʉ", "Kwiinyi", "Kʉsaano", "Kʉsasatʉ"},
		daysAbbreviated:        []string{"Píili", "Táatu", "Íne", "Táano", "Alh", "Ijm", "Móosi"},
		daysNarrow:             []string{"P", "T", "E", "O", "A", "I", "M"},
		daysWide:               []string{"Jumapíiri", "Jumatátu", "Jumaíne", "Jumatáano", "Alamíisi", "Ijumáa", "Jumamóosi"},
		periodsAbbreviated:     []string{"TOO", "MUU"},
		periodsWide:            []string{"TOO", "MUU"},
		erasAbbreviated:        []string{"KSA", "KA"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"Kɨrɨsitʉ sɨ anavyaal", "Kɨrɨsitʉ akavyaalwe"},
		timezones:              map[string]string{"PST": "PST", "CLT": "CLT", "CDT": "CDT", "TMST": "TMST", "ART": "ART", "HEOG": "HEOG", "HEEG": "HEEG", "HKST": "HKST", "GMT": "GMT", "BOT": "BOT", "AKST": "AKST", "HNOG": "HNOG", "ADT": "ADT", "SAST": "SAST", "MDT": "MDT", "HENOMX": "HENOMX", "WAST": "WAST", "HNT": "HNT", "EST": "EST", "ECT": "ECT", "AKDT": "AKDT", "MEZ": "MEZ", "LHDT": "LHDT", "UYST": "UYST", "WITA": "WITA", "CAT": "CAT", "HEPMX": "HEPMX", "MYT": "MYT", "HAT": "HAT", "ACST": "ACST", "∅∅∅": "∅∅∅", "ACWST": "ACWST", "CHADT": "CHADT", "SRT": "SRT", "UYT": "UYT", "ACWDT": "ACWDT", "WART": "WART", "ARST": "ARST", "WIB": "WIB", "HNCU": "HNCU", "AEDT": "AEDT", "LHST": "LHST", "JDT": "JDT", "OESZ": "OESZ", "HADT": "HADT", "VET": "VET", "IST": "IST", "WEZ": "WEZ", "WIT": "WIT", "WAT": "WAT", "COST": "COST", "GFT": "GFT", "ChST": "ChST", "AWST": "AWST", "NZDT": "NZDT", "EAT": "EAT", "HNPM": "HNPM", "CHAST": "CHAST", "CST": "CST", "CLST": "CLST", "AST": "AST", "HNEG": "HNEG", "COT": "COT", "ACDT": "ACDT", "AWDT": "AWDT", "TMT": "TMT", "HECU": "HECU", "NZST": "NZST", "OEZ": "OEZ", "HKT": "HKT", "EDT": "EDT", "GYT": "GYT", "WESZ": "WESZ", "HNPMX": "HNPMX", "PDT": "PDT", "MST": "MST", "MESZ": "MESZ", "JST": "JST", "SGT": "SGT", "HEPM": "HEPM", "WARST": "WARST", "HNNOMX": "HNNOMX", "AEST": "AEST", "BT": "BT", "HAST": "HAST"},
	}
}

// Locale returns the current translators string locale
func (lag *lag) Locale() string {
	return lag.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'lag'
func (lag *lag) PluralsCardinal() []locales.PluralRule {
	return lag.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'lag'
func (lag *lag) PluralsOrdinal() []locales.PluralRule {
	return lag.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'lag'
func (lag *lag) PluralsRange() []locales.PluralRule {
	return lag.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'lag'
func (lag *lag) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if n == 0 {
		return locales.PluralRuleZero
	} else if (i == 0 || i == 1) && n != 0 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'lag'
func (lag *lag) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'lag'
func (lag *lag) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (lag *lag) MonthAbbreviated(month time.Month) string {
	return lag.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (lag *lag) MonthsAbbreviated() []string {
	return lag.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (lag *lag) MonthNarrow(month time.Month) string {
	return lag.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (lag *lag) MonthsNarrow() []string {
	return lag.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (lag *lag) MonthWide(month time.Month) string {
	return lag.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (lag *lag) MonthsWide() []string {
	return lag.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (lag *lag) WeekdayAbbreviated(weekday time.Weekday) string {
	return lag.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (lag *lag) WeekdaysAbbreviated() []string {
	return lag.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (lag *lag) WeekdayNarrow(weekday time.Weekday) string {
	return lag.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (lag *lag) WeekdaysNarrow() []string {
	return lag.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (lag *lag) WeekdayShort(weekday time.Weekday) string {
	return lag.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (lag *lag) WeekdaysShort() []string {
	return lag.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (lag *lag) WeekdayWide(weekday time.Weekday) string {
	return lag.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (lag *lag) WeekdaysWide() []string {
	return lag.daysWide
}

// Decimal returns the decimal point of number
func (lag *lag) Decimal() string {
	return lag.decimal
}

// Group returns the group of number
func (lag *lag) Group() string {
	return lag.group
}

// Group returns the minus sign of number
func (lag *lag) Minus() string {
	return lag.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'lag' and handles both Whole and Real numbers based on 'v'
func (lag *lag) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'lag' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (lag *lag) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'lag'
func (lag *lag) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := lag.currencies[currency]
	l := len(s) + len(symbol) + 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lag.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	for j := len(lag.currencyPositivePrefix) - 1; j >= 0; j-- {
		b = append(b, lag.currencyPositivePrefix[j])
	}

	if num < 0 {
		b = append(b, lag.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, lag.currencyPositiveSuffix...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'lag'
// in accounting notation.
func (lag *lag) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := lag.currencies[currency]
	l := len(s) + len(symbol) + 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lag.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(lag.currencyNegativePrefix) - 1; j >= 0; j-- {
			b = append(b, lag.currencyNegativePrefix[j])
		}

		b = append(b, lag.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(lag.currencyPositivePrefix) - 1; j >= 0; j-- {
			b = append(b, lag.currencyPositivePrefix[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if num < 0 {
		b = append(b, lag.currencyNegativeSuffix...)
	} else {

		b = append(b, lag.currencyPositiveSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'lag'
func (lag *lag) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'lag'
func (lag *lag) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, lag.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'lag'
func (lag *lag) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, lag.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'lag'
func (lag *lag) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, lag.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, lag.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'lag'
func (lag *lag) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'lag'
func (lag *lag) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'lag'
func (lag *lag) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'lag'
func (lag *lag) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lag.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := lag.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
