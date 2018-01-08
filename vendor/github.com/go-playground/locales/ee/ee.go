package ee

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ee struct {
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

// New returns a new instance of translator for the 'ee' locale
func New() locales.Translator {
	return &ee{
		locale:                 "ee",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		timeSeparator:          ":",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AU$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GH₵", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JP¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
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
		timezones:              map[string]string{"SAST": "Anyiehe Africa gaƒoƒome", "CLST": "Tsile dzomeŋɔli gaƒoƒome", "TMST": "Tɛkmenistan dzomeŋɔli gaƒoƒome", "HADT": "Hawaii-Aleutia ŋkekeme gaƒoƒome", "WITA": "WITA", "IST": "IST", "COT": "Kolombia gaƒoƒoɖoanyime", "HAT": "Niufaunɖlanɖ ŋkekeme gaƒoƒome", "ChST": "ChST", "ART": "Argentina gaƒoƒoɖoanyime", "EST": "Ɣedzeƒe America gaƒoƒoɖoanyime", "SGT": "SGT", "CHAST": "CHAST", "HAST": "Hawaii-Aleutia gaƒoƒoɖoanyime", "WART": "Ɣetoɖoƒe Argentina gaƒoƒoɖoanyime", "HNEG": "Ɣedzeƒe Grinlanɖ gaƒoƒoɖoanyime", "HECU": "Kuba ŋkekeme gaƒoƒome", "AWST": "Ɣetoɖoƒe Australia gaƒoƒoɖoanyime", "MESZ": "Titina Europe ŋkekeme gaƒoƒome", "OEZ": "Ɣedzeƒe Europe gaƒoƒoɖoanyime", "WAT": "Ɣetoɖoƒe Afrika gaƒoƒoɖoanyime", "AKDT": "Alaska ŋkekeme gaƒoƒome", "PST": "Pacific gaƒoƒoɖoanyime", "HENOMX": "HENOMX", "MST": "America Todzidukɔwo ƒe gaƒoƒoɖoanyime", "BOT": "Bolivia gaƒoƒome", "AEDT": "Ɣedzeƒe Australia ŋkekeme gaƒoƒome", "HKT": "Hɔng Kɔng gaƒoƒoɖoanyi me", "ARST": "Argentina dzomeŋɔli gaƒoƒome", "WEZ": "Ɣetoɖoƒe Europe gaƒoƒoɖoanyime", "HEPMX": "HEPMX", "WIT": "WIT", "HNNOMX": "HNNOMX", "∅∅∅": "Azores dzomeŋɔli gaƒoƒome", "HEOG": "Ɣetoɖoƒe Grinlanɖ dzomeŋɔli gaƒoƒome", "AEST": "Ɣedzeƒe Australia gaƒoƒoɖoanyime", "CAT": "Titina Afrika gaƒoƒome", "MEZ": "Titina Europe gaƒoƒoɖoanyime", "OESZ": "Ɣedzeƒe Europe ŋkekeme gaƒoƒome", "MDT": "America Todzidukɔwo ƒe ŋkekme gaƒoƒome", "WAST": "Ɣetoɖoƒe Africa ŋkekeme gaƒoƒome", "AKST": "Alaska gaƒoƒoɖoanyime", "WESZ": "Ɣetoɖoƒe Europe ŋkekeme gaƒoƒome", "HNPM": "Saint Pierre kple Mikuelon gaƒoƒoɖoanyime", "CLT": "Tsile gaƒoƒoɖoanyime", "ACST": "Titina Australia gaƒoƒoɖoanyime", "HNPMX": "HNPMX", "CHADT": "CHADT", "BT": "BT", "SRT": "Suriname gaƒoƒome", "UYST": "Uruguai dzomeŋɔli gaƒoƒome", "HKST": "Hɔng Kɔng dzomeŋɔli gaƒoƒome", "JST": "Japan gaƒoƒoɖanyime", "LHDT": "LHDT", "HNOG": "Ɣetoɖoƒe Grinlanɖ gaƒoƒoɖoanyime", "AST": "Atlantic gaƒoƒoɖoanyime", "ADT": "Atlantic ŋkekeme gaƒoƒome", "HEEG": "Ɣedzeƒe Grinlanɖ dzomeŋɔli gaƒoƒome", "HNT": "Niufaunɖlanɖ gaƒoƒoɖoanyime", "WIB": "WIB", "UYT": "Uruguai gaƒoƒoɖoanyime", "WARST": "Ɣetoɖoƒe Argentina dzomeŋɔli gaƒoƒome", "COST": "Kolombia dzomeŋɔli gaƒoƒome", "PDT": "Pacific ŋkekme gaƒoƒome", "HNCU": "Kuba gaƒoƒoɖoanyime", "CST": "Titina America gaƒoƒoɖoanyime", "NZST": "NZST", "ACWDT": "Australia ɣetoɖofe ŋkekeme gaƒoƒome", "LHST": "LHST", "GFT": "Frentsi Guiana gaƒoƒome", "EDT": "Ɣedzeƒe America ŋkekeme gaƒoƒome", "GYT": "Gayana gaƒoƒome", "ACDT": "Titina Australia ŋkekeme gaƒoƒome", "ECT": "Ikuedɔ dzomeŋɔli gaƒoƒome", "CDT": "Titina America ŋkekeme gaƒoƒome", "TMT": "Tɛkmenistan gaƒoƒoɖoanyime", "VET": "Venezuela gaƒoƒome", "EAT": "Ɣedzeƒe Africa gaƒoƒome", "AWDT": "Ɣetoɖoƒe Australia ŋkekeme gaƒoƒome", "NZDT": "NZDT", "JDT": "Japan ŋkekeme gaƒoƒome", "GMT": "Greenwich gaƒoƒome", "HEPM": "Saint Pierre kple Mikuelon ŋkekeme gaƒoƒome", "ACWST": "Australia ɣetoɖofe gaƒoƒoɖoanyime", "MYT": "MYT"},
	}
}

// Locale returns the current translators string locale
func (ee *ee) Locale() string {
	return ee.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ee'
func (ee *ee) PluralsCardinal() []locales.PluralRule {
	return ee.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ee'
func (ee *ee) PluralsOrdinal() []locales.PluralRule {
	return ee.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ee'
func (ee *ee) PluralsRange() []locales.PluralRule {
	return ee.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ee'
func (ee *ee) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ee'
func (ee *ee) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ee'
func (ee *ee) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ee *ee) MonthAbbreviated(month time.Month) string {
	return ee.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ee *ee) MonthsAbbreviated() []string {
	return ee.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ee *ee) MonthNarrow(month time.Month) string {
	return ee.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ee *ee) MonthsNarrow() []string {
	return ee.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ee *ee) MonthWide(month time.Month) string {
	return ee.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ee *ee) MonthsWide() []string {
	return ee.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ee *ee) WeekdayAbbreviated(weekday time.Weekday) string {
	return ee.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ee *ee) WeekdaysAbbreviated() []string {
	return ee.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ee *ee) WeekdayNarrow(weekday time.Weekday) string {
	return ee.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ee *ee) WeekdaysNarrow() []string {
	return ee.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ee *ee) WeekdayShort(weekday time.Weekday) string {
	return ee.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ee *ee) WeekdaysShort() []string {
	return ee.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ee *ee) WeekdayWide(weekday time.Weekday) string {
	return ee.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ee *ee) WeekdaysWide() []string {
	return ee.daysWide
}

// Decimal returns the decimal point of number
func (ee *ee) Decimal() string {
	return ee.decimal
}

// Group returns the group of number
func (ee *ee) Group() string {
	return ee.group
}

// Group returns the minus sign of number
func (ee *ee) Minus() string {
	return ee.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ee' and handles both Whole and Real numbers based on 'v'
func (ee *ee) FmtNumber(num float64, v uint64) string {

	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ee' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ee *ee) FmtPercent(num float64, v uint64) string {
	return strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ee'
func (ee *ee) FmtCurrency(num float64, v uint64, currency currency.Type) string {

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

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ee'
// in accounting notation.
func (ee *ee) FmtAccounting(num float64, v uint64, currency currency.Type) string {

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

// FmtDateShort returns the short date representation of 't' for 'ee'
func (ee *ee) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'ee'
func (ee *ee) FmtDateMedium(t time.Time) string {

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

// FmtDateLong returns the long date representation of 't' for 'ee'
func (ee *ee) FmtDateLong(t time.Time) string {

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

// FmtDateFull returns the full date representation of 't' for 'ee'
func (ee *ee) FmtDateFull(t time.Time) string {

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

// FmtTimeShort returns the short time representation of 't' for 'ee'
func (ee *ee) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, ee.periodsAbbreviated[0]...)
	} else {
		b = append(b, ee.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20, 0x67, 0x61}...)
	b = append(b, []byte{0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ee.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ee'
func (ee *ee) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, ee.periodsAbbreviated[0]...)
	} else {
		b = append(b, ee.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20, 0x67, 0x61}...)
	b = append(b, []byte{0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
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

// FmtTimeLong returns the long time representation of 't' for 'ee'
func (ee *ee) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, ee.periodsAbbreviated[0]...)
	} else {
		b = append(b, ee.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20, 0x67, 0x61}...)
	b = append(b, []byte{0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
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

// FmtTimeFull returns the full time representation of 't' for 'ee'
func (ee *ee) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, ee.periodsAbbreviated[0]...)
	} else {
		b = append(b, ee.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20, 0x67, 0x61}...)
	b = append(b, []byte{0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
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
