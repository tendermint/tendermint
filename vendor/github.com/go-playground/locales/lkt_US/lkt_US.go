package lkt_US

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type lkt_US struct {
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

// New returns a new instance of translator for the 'lkt_US' locale
func New() locales.Translator {
	return &lkt_US{
		locale:                 "lkt_US",
		pluralsCardinal:        []locales.PluralRule{6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ".",
		group:                  ",",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositivePrefix: " ",
		currencyPositiveSuffix: "K",
		currencyNegativePrefix: " ",
		currencyNegativeSuffix: "K",
		monthsWide:             []string{"", "Wiótheȟika Wí", "Thiyóȟeyuŋka Wí", "Ištáwičhayazaŋ Wí", "Pȟežítȟo Wí", "Čhaŋwápetȟo Wí", "Wípazukȟa-wašté Wí", "Čhaŋpȟásapa Wí", "Wasútȟuŋ Wí", "Čhaŋwápeǧi Wí", "Čhaŋwápe-kasná Wí", "Waníyetu Wí", "Tȟahékapšuŋ Wí"},
		daysNarrow:             []string{"A", "W", "N", "Y", "T", "Z", "O"},
		daysWide:               []string{"Aŋpétuwakȟaŋ", "Aŋpétuwaŋži", "Aŋpétunuŋpa", "Aŋpétuyamni", "Aŋpétutopa", "Aŋpétuzaptaŋ", "Owáŋgyužažapi"},
		timezones:              map[string]string{"WARST": "WARST", "WITA": "WITA", "HKT": "HKT", "EST": "EST", "AKST": "AKST", "HNPMX": "HNPMX", "HAST": "HAST", "LHST": "LHST", "EDT": "EDT", "ACWDT": "ACWDT", "GYT": "GYT", "HNCU": "HNCU", "IST": "IST", "HENOMX": "HENOMX", "HEPMX": "HEPMX", "PST": "PST", "CHADT": "CHADT", "HECU": "HECU", "HADT": "HADT", "VET": "VET", "CAT": "CAT", "TMST": "TMST", "JDT": "JDT", "HNOG": "HNOG", "WAT": "WAT", "WESZ": "WESZ", "SGT": "SGT", "BT": "BT", "HNNOMX": "HNNOMX", "ARST": "ARST", "SAST": "SAST", "CLST": "CLST", "WEZ": "WEZ", "MDT": "MDT", "MEZ": "MEZ", "ACST": "ACST", "CHAST": "CHAST", "NZST": "NZST", "ART": "ART", "HNPM": "HNPM", "HEPM": "HEPM", "MST": "MST", "MYT": "MYT", "HNEG": "HNEG", "CST": "CST", "ACWST": "ACWST", "OEZ": "OEZ", "LHDT": "LHDT", "AEST": "AEST", "WIT": "WIT", "NZDT": "NZDT", "HKST": "HKST", "COST": "COST", "AKDT": "AKDT", "GMT": "GMT", "AWST": "AWST", "AWDT": "AWDT", "JST": "JST", "WAST": "WAST", "HEEG": "HEEG", "HAT": "HAT", "AEDT": "AEDT", "HEOG": "HEOG", "EAT": "EAT", "∅∅∅": "∅∅∅", "CDT": "CDT", "UYST": "UYST", "TMT": "TMT", "AST": "AST", "BOT": "BOT", "UYT": "UYT", "OESZ": "OESZ", "WART": "WART", "ADT": "ADT", "SRT": "SRT", "HNT": "HNT", "CLT": "CLT", "GFT": "GFT", "ACDT": "ACDT", "ECT": "ECT", "WIB": "WIB", "COT": "COT", "ChST": "ChST", "PDT": "PDT", "MESZ": "MESZ"},
	}
}

// Locale returns the current translators string locale
func (lkt *lkt_US) Locale() string {
	return lkt.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'lkt_US'
func (lkt *lkt_US) PluralsCardinal() []locales.PluralRule {
	return lkt.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'lkt_US'
func (lkt *lkt_US) PluralsOrdinal() []locales.PluralRule {
	return lkt.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'lkt_US'
func (lkt *lkt_US) PluralsRange() []locales.PluralRule {
	return lkt.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'lkt_US'
func (lkt *lkt_US) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'lkt_US'
func (lkt *lkt_US) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'lkt_US'
func (lkt *lkt_US) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (lkt *lkt_US) MonthAbbreviated(month time.Month) string {
	return lkt.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (lkt *lkt_US) MonthsAbbreviated() []string {
	return nil
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (lkt *lkt_US) MonthNarrow(month time.Month) string {
	return lkt.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (lkt *lkt_US) MonthsNarrow() []string {
	return nil
}

// MonthWide returns the locales wide month given the 'month' provided
func (lkt *lkt_US) MonthWide(month time.Month) string {
	return lkt.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (lkt *lkt_US) MonthsWide() []string {
	return lkt.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (lkt *lkt_US) WeekdayAbbreviated(weekday time.Weekday) string {
	return lkt.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (lkt *lkt_US) WeekdaysAbbreviated() []string {
	return lkt.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (lkt *lkt_US) WeekdayNarrow(weekday time.Weekday) string {
	return lkt.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (lkt *lkt_US) WeekdaysNarrow() []string {
	return lkt.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (lkt *lkt_US) WeekdayShort(weekday time.Weekday) string {
	return lkt.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (lkt *lkt_US) WeekdaysShort() []string {
	return lkt.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (lkt *lkt_US) WeekdayWide(weekday time.Weekday) string {
	return lkt.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (lkt *lkt_US) WeekdaysWide() []string {
	return lkt.daysWide
}

// Decimal returns the decimal point of number
func (lkt *lkt_US) Decimal() string {
	return lkt.decimal
}

// Group returns the group of number
func (lkt *lkt_US) Group() string {
	return lkt.group
}

// Group returns the minus sign of number
func (lkt *lkt_US) Minus() string {
	return lkt.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'lkt_US' and handles both Whole and Real numbers based on 'v'
func (lkt *lkt_US) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'lkt_US' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (lkt *lkt_US) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'lkt_US'
func (lkt *lkt_US) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := lkt.currencies[currency]
	l := len(s) + len(symbol) + 5

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lkt.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	for j := len(lkt.currencyPositivePrefix) - 1; j >= 0; j-- {
		b = append(b, lkt.currencyPositivePrefix[j])
	}

	if num < 0 {
		b = append(b, lkt.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, lkt.currencyPositiveSuffix...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'lkt_US'
// in accounting notation.
func (lkt *lkt_US) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := lkt.currencies[currency]
	l := len(s) + len(symbol) + 5

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lkt.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(lkt.currencyNegativePrefix) - 1; j >= 0; j-- {
			b = append(b, lkt.currencyNegativePrefix[j])
		}

		b = append(b, lkt.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(lkt.currencyPositivePrefix) - 1; j >= 0; j-- {
			b = append(b, lkt.currencyPositivePrefix[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if num < 0 {
		b = append(b, lkt.currencyNegativeSuffix...)
	} else {

		b = append(b, lkt.currencyPositiveSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, lkt.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, lkt.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, lkt.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, lkt.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, lkt.periodsAbbreviated[0]...)
	} else {
		b = append(b, lkt.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, lkt.periodsAbbreviated[0]...)
	} else {
		b = append(b, lkt.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, lkt.periodsAbbreviated[0]...)
	} else {
		b = append(b, lkt.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'lkt_US'
func (lkt *lkt_US) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lkt.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, lkt.periodsAbbreviated[0]...)
	} else {
		b = append(b, lkt.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := lkt.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
