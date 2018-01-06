package lu

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type lu struct {
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

// New returns a new instance of translator for the 'lu' locale
func New() locales.Translator {
	return &lu{
		locale:             "lu",
		pluralsCardinal:    nil,
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		decimal:            ",",
		group:              ".",
		timeSeparator:      ":",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "FC", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "Cio", "Lui", "Lus", "Muu", "Lum", "Luf", "Kab", "Lush", "Lut", "Lun", "Kas", "Cis"},
		monthsNarrow:       []string{"", "C", "L", "L", "M", "L", "L", "K", "L", "L", "L", "K", "C"},
		monthsWide:         []string{"", "Ciongo", "Lùishi", "Lusòlo", "Mùuyà", "Lumùngùlù", "Lufuimi", "Kabàlàshìpù", "Lùshìkà", "Lutongolo", "Lungùdi", "Kaswèkèsè", "Ciswà"},
		daysAbbreviated:    []string{"Lum", "Nko", "Ndy", "Ndg", "Njw", "Ngv", "Lub"},
		daysNarrow:         []string{"L", "N", "N", "N", "N", "N", "L"},
		daysWide:           []string{"Lumingu", "Nkodya", "Ndàayà", "Ndangù", "Njòwa", "Ngòvya", "Lubingu"},
		periodsAbbreviated: []string{"Dinda", "Dilolo"},
		periodsWide:        []string{"Dinda", "Dilolo"},
		erasAbbreviated:    []string{"kmp. Y.K.", "kny. Y. K."},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"Kumpala kwa Yezu Kli", "Kunyima kwa Yezu Kli"},
		timezones:          map[string]string{"HEPM": "HEPM", "AWST": "AWST", "HENOMX": "HENOMX", "OEZ": "OEZ", "ADT": "ADT", "HEOG": "HEOG", "EDT": "EDT", "∅∅∅": "∅∅∅", "ACST": "ACST", "OESZ": "OESZ", "WART": "WART", "EAT": "EAT", "COST": "COST", "EST": "EST", "JDT": "JDT", "LHST": "LHST", "AEST": "AEST", "GFT": "GFT", "MYT": "MYT", "GYT": "GYT", "MST": "MST", "CDT": "CDT", "HAST": "HAST", "WAT": "WAT", "CLST": "CLST", "ACDT": "ACDT", "SRT": "SRT", "TMT": "TMT", "JST": "JST", "HEEG": "HEEG", "WESZ": "WESZ", "MDT": "MDT", "CST": "CST", "ACWST": "ACWST", "WITA": "WITA", "ART": "ART", "AKST": "AKST", "ChST": "ChST", "ECT": "ECT", "GMT": "GMT", "BOT": "BOT", "AST": "AST", "HKST": "HKST", "COT": "COT", "NZST": "NZST", "PDT": "PDT", "UYST": "UYST", "MESZ": "MESZ", "HADT": "HADT", "NZDT": "NZDT", "TMST": "TMST", "ARST": "ARST", "HAT": "HAT", "PST": "PST", "UYT": "UYT", "HNNOMX": "HNNOMX", "CHAST": "CHAST", "CHADT": "CHADT", "HNCU": "HNCU", "HECU": "HECU", "HNPM": "HNPM", "BT": "BT", "HNT": "HNT", "HKT": "HKT", "WAST": "WAST", "ACWDT": "ACWDT", "VET": "VET", "HNOG": "HNOG", "HNEG": "HNEG", "HNPMX": "HNPMX", "WIB": "WIB", "WARST": "WARST", "SAST": "SAST", "WIT": "WIT", "LHDT": "LHDT", "HEPMX": "HEPMX", "AWDT": "AWDT", "AEDT": "AEDT", "CLT": "CLT", "CAT": "CAT", "SGT": "SGT", "MEZ": "MEZ", "IST": "IST", "AKDT": "AKDT", "WEZ": "WEZ"},
	}
}

// Locale returns the current translators string locale
func (lu *lu) Locale() string {
	return lu.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'lu'
func (lu *lu) PluralsCardinal() []locales.PluralRule {
	return lu.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'lu'
func (lu *lu) PluralsOrdinal() []locales.PluralRule {
	return lu.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'lu'
func (lu *lu) PluralsRange() []locales.PluralRule {
	return lu.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'lu'
func (lu *lu) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'lu'
func (lu *lu) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'lu'
func (lu *lu) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (lu *lu) MonthAbbreviated(month time.Month) string {
	return lu.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (lu *lu) MonthsAbbreviated() []string {
	return lu.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (lu *lu) MonthNarrow(month time.Month) string {
	return lu.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (lu *lu) MonthsNarrow() []string {
	return lu.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (lu *lu) MonthWide(month time.Month) string {
	return lu.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (lu *lu) MonthsWide() []string {
	return lu.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (lu *lu) WeekdayAbbreviated(weekday time.Weekday) string {
	return lu.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (lu *lu) WeekdaysAbbreviated() []string {
	return lu.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (lu *lu) WeekdayNarrow(weekday time.Weekday) string {
	return lu.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (lu *lu) WeekdaysNarrow() []string {
	return lu.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (lu *lu) WeekdayShort(weekday time.Weekday) string {
	return lu.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (lu *lu) WeekdaysShort() []string {
	return lu.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (lu *lu) WeekdayWide(weekday time.Weekday) string {
	return lu.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (lu *lu) WeekdaysWide() []string {
	return lu.daysWide
}

// Decimal returns the decimal point of number
func (lu *lu) Decimal() string {
	return lu.decimal
}

// Group returns the group of number
func (lu *lu) Group() string {
	return lu.group
}

// Group returns the minus sign of number
func (lu *lu) Minus() string {
	return lu.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'lu' and handles both Whole and Real numbers based on 'v'
func (lu *lu) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 1 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, lu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, lu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'lu' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (lu *lu) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'lu'
func (lu *lu) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := lu.currencies[currency]
	l := len(s) + len(symbol) + 1 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, lu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, lu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, lu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'lu'
// in accounting notation.
func (lu *lu) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := lu.currencies[currency]
	l := len(s) + len(symbol) + 1 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, lu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, lu.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, lu.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, lu.decimal...)
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

// FmtDateShort returns the short date representation of 't' for 'lu'
func (lu *lu) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'lu'
func (lu *lu) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, lu.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'lu'
func (lu *lu) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, lu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'lu'
func (lu *lu) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, lu.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, lu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'lu'
func (lu *lu) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'lu'
func (lu *lu) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'lu'
func (lu *lu) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'lu'
func (lu *lu) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, lu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := lu.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
