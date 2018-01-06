package ig_NG

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ig_NG struct {
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

// New returns a new instance of translator for the 'ig_NG' locale
func New() locales.Translator {
	return &ig_NG{
		locale:             "ig_NG",
		pluralsCardinal:    []locales.PluralRule{6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		decimal:            "٫",
		group:              "٬",
		perMille:           "؉",
		timeSeparator:      ":",
		inifinity:          "∞",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:      " ",
		monthsAbbreviated:  []string{"", "Jen", "Feb", "Maa", "Epr", "Mee", "Juu", "Jul", "Ọgọ", "Sep", "Ọkt", "Nov", "Dis"},
		monthsNarrow:       []string{"", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
		monthsWide:         []string{"", "Jenụwarị", "Febrụwarị", "Maachị", "Eprel", "Mee", "Juun", "Julaị", "Ọgọọst", "Septemba", "Ọktoba", "Novemba", "Disemba"},
		daysAbbreviated:    []string{"Ụka", "Mọn", "Tiu", "Wen", "Tọọ", "Fraị", "Sat"},
		daysNarrow:         []string{"S", "M", "T", "W", "T", "F", "S"},
		daysShort:          []string{"Ụka", "Mọn", "Tiu", "Wen", "Tọọ", "Fraị", "Sat"},
		daysWide:           []string{"Mbọsị Ụka", "Mọnde", "Tiuzdee", "Wenezdee", "Tọọzdee", "Fraịdee", "Satọdee"},
		periodsAbbreviated: []string{"A.M.", "P.M."},
		periodsNarrow:      []string{"A.M.", "P.M."},
		periodsWide:        []string{"A.M.", "P.M."},
		erasAbbreviated:    []string{"T.K.", "A.K."},
		erasNarrow:         []string{"T.K.", "A.K."},
		erasWide:           []string{"Tupu Kristi", "Afọ Kristi"},
		timezones:          map[string]string{"AEDT": "AEDT", "CDT": "CDT", "TMT": "TMT", "HNOG": "HNOG", "HEOG": "HEOG", "MYT": "MYT", "LHDT": "LHDT", "CLST": "CLST", "ACDT": "ACDT", "NZDT": "NZDT", "VET": "VET", "WITA": "WITA", "OEZ": "OEZ", "AKST": "AKST", "MDT": "MDT", "EAT": "EAT", "HAT": "HAT", "SGT": "SGT", "CAT": "CAT", "HNPMX": "HNPMX", "CHAST": "CHAST", "ART": "ART", "HEEG": "HEEG", "MEZ": "MEZ", "HNT": "HNT", "WESZ": "WESZ", "HNCU": "HNCU", "AWST": "AWST", "AEST": "AEST", "HNEG": "HNEG", "WAT": "WAT", "WAST": "WAST", "HKST": "HKST", "EDT": "EDT", "WEZ": "WEZ", "CST": "CST", "IST": "IST", "∅∅∅": "∅∅∅", "ACWST": "ACWST", "HEPMX": "HEPMX", "NZST": "NZST", "BOT": "BOT", "SRT": "SRT", "WIT": "WIT", "HKT": "HKT", "WIB": "WIB", "AWDT": "AWDT", "MESZ": "MESZ", "OESZ": "OESZ", "ACST": "ACST", "GMT": "GMT", "UYST": "UYST", "AST": "AST", "HEPM": "HEPM", "ACWDT": "ACWDT", "CLT": "CLT", "COT": "COT", "ChST": "ChST", "WART": "WART", "JST": "JST", "LHST": "LHST", "ADT": "ADT", "EST": "EST", "HECU": "HECU", "JDT": "JDT", "GFT": "GFT", "AKDT": "AKDT", "PST": "PST", "MST": "MST", "UYT": "UYT", "HADT": "HADT", "WARST": "WARST", "ARST": "ARST", "ECT": "ECT", "PDT": "PDT", "CHADT": "CHADT", "TMST": "TMST", "HNNOMX": "HNNOMX", "SAST": "SAST", "GYT": "GYT", "BT": "BT", "HAST": "HAST", "HENOMX": "HENOMX", "COST": "COST", "HNPM": "HNPM"},
	}
}

// Locale returns the current translators string locale
func (ig *ig_NG) Locale() string {
	return ig.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ig_NG'
func (ig *ig_NG) PluralsCardinal() []locales.PluralRule {
	return ig.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ig_NG'
func (ig *ig_NG) PluralsOrdinal() []locales.PluralRule {
	return ig.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ig_NG'
func (ig *ig_NG) PluralsRange() []locales.PluralRule {
	return ig.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ig_NG'
func (ig *ig_NG) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ig_NG'
func (ig *ig_NG) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ig_NG'
func (ig *ig_NG) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ig *ig_NG) MonthAbbreviated(month time.Month) string {
	return ig.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ig *ig_NG) MonthsAbbreviated() []string {
	return ig.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ig *ig_NG) MonthNarrow(month time.Month) string {
	return ig.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ig *ig_NG) MonthsNarrow() []string {
	return ig.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ig *ig_NG) MonthWide(month time.Month) string {
	return ig.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ig *ig_NG) MonthsWide() []string {
	return ig.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ig *ig_NG) WeekdayAbbreviated(weekday time.Weekday) string {
	return ig.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ig *ig_NG) WeekdaysAbbreviated() []string {
	return ig.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ig *ig_NG) WeekdayNarrow(weekday time.Weekday) string {
	return ig.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ig *ig_NG) WeekdaysNarrow() []string {
	return ig.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ig *ig_NG) WeekdayShort(weekday time.Weekday) string {
	return ig.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ig *ig_NG) WeekdaysShort() []string {
	return ig.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ig *ig_NG) WeekdayWide(weekday time.Weekday) string {
	return ig.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ig *ig_NG) WeekdaysWide() []string {
	return ig.daysWide
}

// Decimal returns the decimal point of number
func (ig *ig_NG) Decimal() string {
	return ig.decimal
}

// Group returns the group of number
func (ig *ig_NG) Group() string {
	return ig.group
}

// Group returns the minus sign of number
func (ig *ig_NG) Minus() string {
	return ig.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ig_NG' and handles both Whole and Real numbers based on 'v'
func (ig *ig_NG) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			for j := len(ig.decimal) - 1; j >= 0; j-- {
				b = append(b, ig.decimal[j])
			}
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ig.group) - 1; j >= 0; j-- {
					b = append(b, ig.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ig.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ig_NG' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ig *ig_NG) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 4
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			for j := len(ig.decimal) - 1; j >= 0; j-- {
				b = append(b, ig.decimal[j])
			}
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ig.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, ig.percentSuffix...)

	b = append(b, ig.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ig_NG'
func (ig *ig_NG) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ig.currencies[currency]
	l := len(s) + len(symbol) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			for j := len(ig.decimal) - 1; j >= 0; j-- {
				b = append(b, ig.decimal[j])
			}
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ig.group) - 1; j >= 0; j-- {
					b = append(b, ig.group[j])
				}
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
		b = append(b, ig.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ig.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ig_NG'
// in accounting notation.
func (ig *ig_NG) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ig.currencies[currency]
	l := len(s) + len(symbol) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			for j := len(ig.decimal) - 1; j >= 0; j-- {
				b = append(b, ig.decimal[j])
			}
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ig.group) - 1; j >= 0; j-- {
					b = append(b, ig.group[j])
				}
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

		b = append(b, ig.minus[0])

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
			b = append(b, ig.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ig.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ig.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ig.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ig.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ig_NG'
func (ig *ig_NG) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ig.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ig.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
