package wae

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type wae struct {
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

// New returns a new instance of translator for the 'wae' locale
func New() locales.Translator {
	return &wae{
		locale:                 "wae",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  "’",
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositivePrefix: " ",
		currencyPositiveSuffix: "K",
		currencyNegativePrefix: " ",
		currencyNegativeSuffix: "K",
		monthsAbbreviated:      []string{"", "Jen", "Hor", "Mär", "Abr", "Mei", "Brá", "Hei", "Öig", "Her", "Wím", "Win", "Chr"},
		monthsNarrow:           []string{"", "J", "H", "M", "A", "M", "B", "H", "Ö", "H", "W", "W", "C"},
		monthsWide:             []string{"", "Jenner", "Hornig", "Märze", "Abrille", "Meije", "Bráčet", "Heiwet", "Öigšte", "Herbštmánet", "Wímánet", "Wintermánet", "Chrištmánet"},
		daysAbbreviated:        []string{"Sun", "Män", "Ziš", "Mit", "Fró", "Fri", "Sam"},
		daysNarrow:             []string{"S", "M", "Z", "M", "F", "F", "S"},
		daysWide:               []string{"Sunntag", "Mäntag", "Zištag", "Mittwuč", "Fróntag", "Fritag", "Samštag"},
		erasAbbreviated:        []string{"v. Chr.", "n. Chr"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"", ""},
		timezones:              map[string]string{"WIB": "WIB", "HNPM": "HNPM", "JDT": "JDT", "LHST": "LHST", "ADT": "Atlantiši Summerzit", "WAT": "WAT", "AWST": "AWST", "MEZ": "Mitteleuropäiši Standardzit", "NZST": "NZST", "WARST": "WARST", "HENOMX": "HENOMX", "ART": "ART", "JST": "JST", "ACST": "ACST", "ChST": "ChST", "PDT": "PDT", "AST": "Atlantiši Standardzit", "ECT": "ECT", "HNPMX": "HNPMX", "LHDT": "LHDT", "PST": "PST", "SRT": "SRT", "CDT": "CDT", "ACWDT": "ACWDT", "OESZ": "Ošteuropäiši Summerzit", "HKT": "HKT", "SGT": "SGT", "HEOG": "HEOG", "CLT": "CLT", "CAT": "CAT", "HEPMX": "HEPMX", "BOT": "BOT", "COT": "COT", "HKST": "HKST", "AKST": "AKST", "MST": "MST", "NZDT": "NZDT", "AEST": "AEST", "HNEG": "HNEG", "HAT": "HAT", "GYT": "GYT", "EST": "EST", "GMT": "GMT", "CST": "CST", "WIT": "WIT", "MESZ": "Mitteleuropäiši Summerzit", "WAST": "WAST", "COST": "COST", "WITA": "WITA", "HNOG": "HNOG", "AEDT": "AEDT", "GFT": "GFT", "HEPM": "HEPM", "CHADT": "CHADT", "HECU": "HECU", "ACWST": "ACWST", "UYST": "UYST", "TMT": "TMT", "CLST": "CLST", "EDT": "EDT", "ACDT": "ACDT", "BT": "BT", "HAST": "HAST", "WART": "WART", "OEZ": "Ošteuropäiši Standardzit", "IST": "IST", "HNT": "HNT", "∅∅∅": "∅∅∅", "MDT": "MDT", "AWDT": "AWDT", "UYT": "UYT", "AKDT": "AKDT", "WEZ": "Wešteuropäiši Standardzit", "HNCU": "HNCU", "TMST": "TMST", "HADT": "HADT", "ARST": "ARST", "EAT": "EAT", "SAST": "SAST", "WESZ": "Wešteuropäiši Summerzit", "CHAST": "CHAST", "MYT": "MYT", "VET": "VET", "HNNOMX": "HNNOMX", "HEEG": "HEEG"},
	}
}

// Locale returns the current translators string locale
func (wae *wae) Locale() string {
	return wae.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'wae'
func (wae *wae) PluralsCardinal() []locales.PluralRule {
	return wae.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'wae'
func (wae *wae) PluralsOrdinal() []locales.PluralRule {
	return wae.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'wae'
func (wae *wae) PluralsRange() []locales.PluralRule {
	return wae.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'wae'
func (wae *wae) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'wae'
func (wae *wae) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'wae'
func (wae *wae) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (wae *wae) MonthAbbreviated(month time.Month) string {
	return wae.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (wae *wae) MonthsAbbreviated() []string {
	return wae.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (wae *wae) MonthNarrow(month time.Month) string {
	return wae.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (wae *wae) MonthsNarrow() []string {
	return wae.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (wae *wae) MonthWide(month time.Month) string {
	return wae.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (wae *wae) MonthsWide() []string {
	return wae.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (wae *wae) WeekdayAbbreviated(weekday time.Weekday) string {
	return wae.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (wae *wae) WeekdaysAbbreviated() []string {
	return wae.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (wae *wae) WeekdayNarrow(weekday time.Weekday) string {
	return wae.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (wae *wae) WeekdaysNarrow() []string {
	return wae.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (wae *wae) WeekdayShort(weekday time.Weekday) string {
	return wae.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (wae *wae) WeekdaysShort() []string {
	return wae.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (wae *wae) WeekdayWide(weekday time.Weekday) string {
	return wae.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (wae *wae) WeekdaysWide() []string {
	return wae.daysWide
}

// Decimal returns the decimal point of number
func (wae *wae) Decimal() string {
	return wae.decimal
}

// Group returns the group of number
func (wae *wae) Group() string {
	return wae.group
}

// Group returns the minus sign of number
func (wae *wae) Minus() string {
	return wae.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'wae' and handles both Whole and Real numbers based on 'v'
func (wae *wae) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'wae' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (wae *wae) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'wae'
func (wae *wae) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := wae.currencies[currency]
	l := len(s) + len(symbol) + 4

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, wae.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	for j := len(wae.currencyPositivePrefix) - 1; j >= 0; j-- {
		b = append(b, wae.currencyPositivePrefix[j])
	}

	if num < 0 {
		b = append(b, wae.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, wae.currencyPositiveSuffix...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'wae'
// in accounting notation.
func (wae *wae) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := wae.currencies[currency]
	l := len(s) + len(symbol) + 4

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, wae.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(wae.currencyNegativePrefix) - 1; j >= 0; j-- {
			b = append(b, wae.currencyNegativePrefix[j])
		}

		b = append(b, wae.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(wae.currencyPositivePrefix) - 1; j >= 0; j-- {
			b = append(b, wae.currencyPositivePrefix[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if num < 0 {
		b = append(b, wae.currencyNegativeSuffix...)
	} else {

		b = append(b, wae.currencyPositiveSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'wae'
func (wae *wae) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'wae'
func (wae *wae) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, wae.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'wae'
func (wae *wae) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, wae.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'wae'
func (wae *wae) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, wae.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, wae.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'wae'
func (wae *wae) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'wae'
func (wae *wae) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'wae'
func (wae *wae) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'wae'
func (wae *wae) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	return string(b)
}
