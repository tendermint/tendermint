package ee_TG

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ee_TG struct {
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

// New returns a new instance of translator for the 'ee_TG' locale
func New() locales.Translator {
	return &ee_TG{
		locale:                 "ee_TG",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: ")",
		monthsAbbreviated:      []string{"", "dzv", "dzd", "ted", "afɔ", "dam", "mas", "sia", "dea", "any", "kel", "ade", "dzm"},
		monthsNarrow:           []string{"", "d", "d", "t", "a", "d", "m", "s", "d", "a", "k", "a", "d"},
		monthsWide:             []string{"", "dzove", "dzodze", "tedoxe", "afɔfĩe", "dama", "masa", "siamlɔm", "deasiamime", "anyɔnyɔ", "kele", "adeɛmekpɔxe", "dzome"},
		daysAbbreviated:        []string{"kɔs", "dzo", "bla", "kuɖ", "yaw", "fiɖ", "mem"},
		daysNarrow:             []string{"k", "d", "b", "k", "y", "f", "m"},
		daysShort:              []string{"kɔs", "dzo", "bla", "kuɖ", "yaw", "fiɖ", "mem"},
		daysWide:               []string{"kɔsiɖa", "dzoɖa", "blaɖa", "kuɖa", "yawoɖa", "fiɖa", "memleɖa"},
		periodsAbbreviated:     []string{"ŋdi", "ɣetrɔ"},
		periodsNarrow:          []string{"ŋ", "ɣ"},
		periodsWide:            []string{"ŋdi", "ɣetrɔ"},
		erasAbbreviated:        []string{"hY", "Yŋ"},
		erasNarrow:             []string{"hY", "Yŋ"},
		erasWide:               []string{"Hafi Yesu Va Do ŋgɔ", "Yesu Ŋɔli"},
		timezones:              map[string]string{"CLST": "Tsile dzomeŋɔli gaƒoƒome", "ECT": "Ikuedɔ dzomeŋɔli gaƒoƒome", "GMT": "Greenwich gaƒoƒome", "CST": "Titina America gaƒoƒoɖoanyime", "TMT": "Tɛkmenistan gaƒoƒoɖoanyime", "AEST": "Ɣedzeƒe Australia gaƒoƒoɖoanyime", "HNEG": "Ɣedzeƒe Grinlanɖ gaƒoƒoɖoanyime", "CLT": "Tsile gaƒoƒoɖoanyime", "PST": "Pacific gaƒoƒoɖoanyime", "WIB": "WIB", "BOT": "Bolivia gaƒoƒome", "WIT": "WIT", "ACWST": "Australia ɣetoɖofe gaƒoƒoɖoanyime", "HEEG": "Ɣedzeƒe Grinlanɖ dzomeŋɔli gaƒoƒome", "HKT": "Hɔng Kɔng gaƒoƒoɖoanyi me", "HAT": "Niufaunɖlanɖ ŋkekeme gaƒoƒome", "WAT": "Ɣetoɖoƒe Afrika gaƒoƒoɖoanyime", "GFT": "Frentsi Guiana gaƒoƒome", "HEPM": "Saint Pierre kple Mikuelon ŋkekeme gaƒoƒome", "UYST": "Uruguai dzomeŋɔli gaƒoƒome", "ACWDT": "Australia ɣetoɖofe ŋkekeme gaƒoƒome", "VET": "Venezuela gaƒoƒome", "OESZ": "Ɣedzeƒe Europe ŋkekeme gaƒoƒome", "AST": "Atlantic gaƒoƒoɖoanyime", "CDT": "Titina America ŋkekeme gaƒoƒome", "WARST": "Ɣetoɖoƒe Argentina dzomeŋɔli gaƒoƒome", "AKDT": "Alaska ŋkekeme gaƒoƒome", "SRT": "Suriname gaƒoƒome", "∅∅∅": "Eker dzomeŋɔli gaƒoƒome", "PDT": "Pacific ŋkekme gaƒoƒome", "WEZ": "Ɣetoɖoƒe Europe gaƒoƒoɖoanyime", "HNCU": "Kuba gaƒoƒoɖoanyime", "HAST": "Hawaii-Aleutia gaƒoƒoɖoanyime", "MEZ": "Titina Europe gaƒoƒoɖoanyime", "WART": "Ɣetoɖoƒe Argentina gaƒoƒoɖoanyime", "WITA": "WITA", "ACDT": "Titina Australia ŋkekeme gaƒoƒome", "HEPMX": "HEPMX", "TMST": "Tɛkmenistan dzomeŋɔli gaƒoƒome", "ARST": "Argentina dzomeŋɔli gaƒoƒome", "HNOG": "Ɣetoɖoƒe Grinlanɖ gaƒoƒoɖoanyime", "ACST": "Titina Australia gaƒoƒoɖoanyime", "WESZ": "Ɣetoɖoƒe Europe ŋkekeme gaƒoƒome", "NZST": "NZST", "SAST": "Anyiehe Africa gaƒoƒome", "COST": "Kolombia dzomeŋɔli gaƒoƒome", "SGT": "SGT", "HECU": "Kuba ŋkekeme gaƒoƒome", "HNNOMX": "HNNOMX", "HENOMX": "HENOMX", "ART": "Argentina gaƒoƒoɖoanyime", "AKST": "Alaska gaƒoƒoɖoanyime", "HNPM": "Saint Pierre kple Mikuelon gaƒoƒoɖoanyime", "IST": "IST", "HNT": "Niufaunɖlanɖ gaƒoƒoɖoanyime", "GYT": "Gayana gaƒoƒome", "HNPMX": "HNPMX", "CHADT": "CHADT", "MDT": "Makau ŋkekeme gaƒoƒome", "MYT": "MYT", "NZDT": "NZDT", "MESZ": "Titina Europe ŋkekeme gaƒoƒome", "EDT": "Ɣedzeƒe America ŋkekeme gaƒoƒome", "UYT": "Uruguai gaƒoƒoɖoanyime", "LHDT": "LHDT", "EAT": "Ɣedzeƒe Africa gaƒoƒome", "BT": "BT", "LHST": "LHST", "EST": "Ɣedzeƒe America gaƒoƒoɖoanyime", "ChST": "ChST", "CAT": "Titina Afrika gaƒoƒome", "MST": "Makau gaƒoƒoɖoanyime", "JST": "Japan gaƒoƒoɖanyime", "AEDT": "Ɣedzeƒe Australia ŋkekeme gaƒoƒome", "ADT": "Atlantic ŋkekeme gaƒoƒome", "HKST": "Hɔng Kɔng dzomeŋɔli gaƒoƒome", "COT": "Kolombia gaƒoƒoɖoanyime", "AWST": "Ɣetoɖoƒe Australia gaƒoƒoɖoanyime", "AWDT": "Ɣetoɖoƒe Australia ŋkekeme gaƒoƒome", "HADT": "Hawaii-Aleutia ŋkekeme gaƒoƒome", "JDT": "Japan ŋkekeme gaƒoƒome", "OEZ": "Ɣedzeƒe Europe gaƒoƒoɖoanyime", "HEOG": "Ɣetoɖoƒe Grinlanɖ dzomeŋɔli gaƒoƒome", "WAST": "Ɣetoɖoƒe Africa ŋkekeme gaƒoƒome", "CHAST": "CHAST"},
	}
}

// Locale returns the current translators string locale
func (ee *ee_TG) Locale() string {
	return ee.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ee_TG'
func (ee *ee_TG) PluralsCardinal() []locales.PluralRule {
	return ee.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ee_TG'
func (ee *ee_TG) PluralsOrdinal() []locales.PluralRule {
	return ee.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ee_TG'
func (ee *ee_TG) PluralsRange() []locales.PluralRule {
	return ee.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ee_TG'
func (ee *ee_TG) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ee_TG'
func (ee *ee_TG) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ee_TG'
func (ee *ee_TG) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ee *ee_TG) MonthAbbreviated(month time.Month) string {
	return ee.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ee *ee_TG) MonthsAbbreviated() []string {
	return ee.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ee *ee_TG) MonthNarrow(month time.Month) string {
	return ee.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ee *ee_TG) MonthsNarrow() []string {
	return ee.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ee *ee_TG) MonthWide(month time.Month) string {
	return ee.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ee *ee_TG) MonthsWide() []string {
	return ee.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ee *ee_TG) WeekdayAbbreviated(weekday time.Weekday) string {
	return ee.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ee *ee_TG) WeekdaysAbbreviated() []string {
	return ee.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ee *ee_TG) WeekdayNarrow(weekday time.Weekday) string {
	return ee.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ee *ee_TG) WeekdaysNarrow() []string {
	return ee.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ee *ee_TG) WeekdayShort(weekday time.Weekday) string {
	return ee.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ee *ee_TG) WeekdaysShort() []string {
	return ee.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ee *ee_TG) WeekdayWide(weekday time.Weekday) string {
	return ee.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ee *ee_TG) WeekdaysWide() []string {
	return ee.daysWide
}

// Decimal returns the decimal point of number
func (ee *ee_TG) Decimal() string {
	return ee.decimal
}

// Group returns the group of number
func (ee *ee_TG) Group() string {
	return ee.group
}

// Group returns the minus sign of number
func (ee *ee_TG) Minus() string {
	return ee.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ee_TG' and handles both Whole and Real numbers based on 'v'
func (ee *ee_TG) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ee_TG' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ee *ee_TG) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ee_TG'
func (ee *ee_TG) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ee.currencies[currency]
	l := len(s) + len(symbol) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ee.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ee.group[0])
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
		b = append(b, ee.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ee.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ee_TG'
// in accounting notation.
func (ee *ee_TG) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ee.currencies[currency]
	l := len(s) + len(symbol) + 2
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ee.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ee.group[0])
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

		b = append(b, ee.currencyNegativePrefix[0])

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
			b = append(b, ee.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, ee.currencyNegativeSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ee.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x6c, 0x69, 0x61}...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ee.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x6c, 0x69, 0x61}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ee.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, ee.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x6c, 0x69, 0x61}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ee_TG'
func (ee *ee_TG) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ee.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ee.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
