package ff_CM

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ff_CM struct {
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
	currencyPositiveSuffix string
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

// New returns a new instance of translator for the 'ff_CM' locale
func New() locales.Translator {
	return &ff_CM{
		locale:                 "ff_CM",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  " ",
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "sii", "col", "mbo", "see", "duu", "kor", "mor", "juk", "slt", "yar", "jol", "bow"},
		monthsNarrow:           []string{"", "s", "c", "m", "s", "d", "k", "m", "j", "s", "y", "j", "b"},
		monthsWide:             []string{"", "siilo", "colte", "mbooy", "seeɗto", "duujal", "korse", "morso", "juko", "siilto", "yarkomaa", "jolal", "bowte"},
		daysAbbreviated:        []string{"dew", "aaɓ", "maw", "nje", "naa", "mwd", "hbi"},
		daysNarrow:             []string{"d", "a", "m", "n", "n", "m", "h"},
		daysWide:               []string{"dewo", "aaɓnde", "mawbaare", "njeslaare", "naasaande", "mawnde", "hoore-biir"},
		periodsAbbreviated:     []string{"subaka", "kikiiɗe"},
		periodsWide:            []string{"subaka", "kikiiɗe"},
		erasAbbreviated:        []string{"H-I", "C-I"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"Hade Iisa", "Caggal Iisa"},
		timezones:              map[string]string{"HNPMX": "HNPMX", "MST": "MST", "ACWDT": "ACWDT", "HAST": "HAST", "HADT": "HADT", "HNNOMX": "HNNOMX", "CHADT": "CHADT", "BOT": "BOT", "CST": "CST", "HKT": "HKT", "CLT": "CLT", "COT": "COT", "CAT": "CAT", "AWDT": "AWDT", "JST": "JST", "TMST": "TMST", "OESZ": "OESZ", "ARST": "ARST", "HNEG": "HNEG", "HNCU": "HNCU", "HECU": "HECU", "UYT": "UYT", "HENOMX": "HENOMX", "EST": "EST", "ACDT": "ACDT", "GMT": "GMT", "WIB": "WIB", "EAT": "EAT", "WESZ": "WESZ", "HEPMX": "HEPMX", "WIT": "WIT", "MEZ": "MEZ", "AST": "AST", "HEEG": "HEEG", "MDT": "MDT", "UYST": "UYST", "WART": "WART", "AKDT": "AKDT", "PDT": "PDT", "AWST": "AWST", "SRT": "SRT", "WARST": "WARST", "CLST": "CLST", "GYT": "GYT", "CHAST": "CHAST", "NZDT": "NZDT", "JDT": "JDT", "WITA": "WITA", "HNOG": "HNOG", "HEOG": "HEOG", "HKST": "HKST", "ECT": "ECT", "WEZ": "WEZ", "HEPM": "HEPM", "BT": "BT", "VET": "VET", "EDT": "EDT", "OEZ": "OEZ", "AEST": "AEST", "ART": "ART", "∅∅∅": "∅∅∅", "SGT": "SGT", "ACWST": "ACWST", "NZST": "NZST", "IST": "IST", "ADT": "ADT", "AEDT": "AEDT", "AKST": "AKST", "ACST": "ACST", "HNPM": "HNPM", "MYT": "MYT", "MESZ": "MESZ", "TMT": "TMT", "LHST": "LHST", "HAT": "HAT", "COST": "COST", "GFT": "GFT", "ChST": "ChST", "PST": "PST", "CDT": "CDT", "LHDT": "LHDT", "SAST": "SAST", "WAT": "WAT", "WAST": "WAST", "HNT": "HNT"},
	}
}

// Locale returns the current translators string locale
func (ff *ff_CM) Locale() string {
	return ff.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ff_CM'
func (ff *ff_CM) PluralsCardinal() []locales.PluralRule {
	return ff.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ff_CM'
func (ff *ff_CM) PluralsOrdinal() []locales.PluralRule {
	return ff.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ff_CM'
func (ff *ff_CM) PluralsRange() []locales.PluralRule {
	return ff.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ff_CM'
func (ff *ff_CM) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 0 || i == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ff_CM'
func (ff *ff_CM) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ff_CM'
func (ff *ff_CM) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ff *ff_CM) MonthAbbreviated(month time.Month) string {
	return ff.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ff *ff_CM) MonthsAbbreviated() []string {
	return ff.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ff *ff_CM) MonthNarrow(month time.Month) string {
	return ff.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ff *ff_CM) MonthsNarrow() []string {
	return ff.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ff *ff_CM) MonthWide(month time.Month) string {
	return ff.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ff *ff_CM) MonthsWide() []string {
	return ff.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ff *ff_CM) WeekdayAbbreviated(weekday time.Weekday) string {
	return ff.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ff *ff_CM) WeekdaysAbbreviated() []string {
	return ff.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ff *ff_CM) WeekdayNarrow(weekday time.Weekday) string {
	return ff.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ff *ff_CM) WeekdaysNarrow() []string {
	return ff.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ff *ff_CM) WeekdayShort(weekday time.Weekday) string {
	return ff.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ff *ff_CM) WeekdaysShort() []string {
	return ff.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ff *ff_CM) WeekdayWide(weekday time.Weekday) string {
	return ff.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ff *ff_CM) WeekdaysWide() []string {
	return ff.daysWide
}

// Decimal returns the decimal point of number
func (ff *ff_CM) Decimal() string {
	return ff.decimal
}

// Group returns the group of number
func (ff *ff_CM) Group() string {
	return ff.group
}

// Group returns the minus sign of number
func (ff *ff_CM) Minus() string {
	return ff.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ff_CM' and handles both Whole and Real numbers based on 'v'
func (ff *ff_CM) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ff_CM' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ff *ff_CM) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ff_CM'
func (ff *ff_CM) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ff.currencies[currency]
	l := len(s) + len(symbol) + 3 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ff.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ff.group) - 1; j >= 0; j-- {
					b = append(b, ff.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ff.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ff.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, ff.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ff_CM'
// in accounting notation.
func (ff *ff_CM) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ff.currencies[currency]
	l := len(s) + len(symbol) + 3 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ff.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(ff.group) - 1; j >= 0; j-- {
					b = append(b, ff.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, ff.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ff.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, ff.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, ff.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ff.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ff.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ff.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ff.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ff_CM'
func (ff *ff_CM) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ff.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ff.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
