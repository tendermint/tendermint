package zh_Hant

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type zh_Hant struct {
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

// New returns a new instance of translator for the 'zh_Hant' locale
func New() locales.Translator {
	return &zh_Hant{
		locale:             "zh_Hant",
		pluralsCardinal:    []locales.PluralRule{6},
		pluralsOrdinal:     []locales.PluralRule{6},
		pluralsRange:       []locales.PluralRule{6},
		decimal:            ".",
		group:              ",",
		minus:              "-",
		percent:            "%",
		perMille:           "‰",
		timeSeparator:      ":",
		inifinity:          "∞",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AU$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "￦", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
		monthsNarrow:       []string{"", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
		monthsWide:         []string{"", "1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
		daysAbbreviated:    []string{"週日", "週一", "週二", "週三", "週四", "週五", "週六"},
		daysNarrow:         []string{"日", "一", "二", "三", "四", "五", "六"},
		daysShort:          []string{"日", "一", "二", "三", "四", "五", "六"},
		daysWide:           []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"},
		periodsAbbreviated: []string{"上午", "下午"},
		periodsNarrow:      []string{"上午", "下午"},
		periodsWide:        []string{"上午", "下午"},
		erasAbbreviated:    []string{"西元前", "西元"},
		erasNarrow:         []string{"西元前", "西元"},
		erasWide:           []string{"西元前", "西元"},
		timezones:          map[string]string{"∅∅∅": "巴西利亞夏令時間", "AWST": "澳洲西部標準時間", "AEDT": "澳洲東部夏令時間", "ADT": "大西洋夏令時間", "WIB": "印尼西部時間", "CHADT": "查坦群島夏令時間", "TMT": "土庫曼標準時間", "EAT": "東非時間", "MEZ": "中歐標準時間", "WARST": "阿根廷西部夏令時間", "WAT": "西非標準時間", "HNPMX": "墨西哥太平洋標準時間", "HECU": "古巴夏令時間", "HNPM": "聖皮埃爾和密克隆群島標準時間", "HEPM": "聖皮埃爾和密克隆群島夏令時間", "AWDT": "澳洲西部夏令時間", "HEEG": "格陵蘭東部夏令時間", "HKT": "香港標準時間", "EDT": "東部夏令時間", "HEPMX": "墨西哥太平洋夏令時間", "PST": "太平洋標準時間", "UYT": "烏拉圭標準時間", "COT": "哥倫比亞標準時間", "ChST": "查莫洛時間", "OEZ": "東歐標準時間", "AST": "大西洋標準時間", "SAST": "南非標準時間", "HNEG": "格陵蘭東部標準時間", "BOT": "玻利維亞時間", "CST": "中部標準時間", "TMST": "土庫曼夏令時間", "EST": "東部標準時間", "CLST": "智利夏令時間", "GYT": "蓋亞那時間", "ACDT": "澳洲中部夏令時間", "MST": "澳門標準時間", "LHDT": "豪勳爵島夏令時間", "ARST": "阿根廷夏令時間", "HNOG": "格陵蘭西部標準時間", "CLT": "智利標準時間", "SGT": "新加坡標準時間", "ART": "阿根廷標準時間", "COST": "哥倫比亞夏令時間", "GFT": "法屬圭亞那時間", "BT": "不丹時間", "WART": "阿根廷西部標準時間", "HENOMX": "墨西哥西北部夏令時間", "JST": "日本標準時間", "AEST": "澳洲東部標準時間", "AKST": "阿拉斯加標準時間", "HKST": "香港夏令時間", "AKDT": "阿拉斯加夏令時間", "WEZ": "西歐標準時間", "HNCU": "古巴標準時間", "SRT": "蘇利南時間", "ACWDT": "澳洲中西部夏令時間", "WITA": "印尼中部時間", "HEOG": "格陵蘭西部夏令時間", "CDT": "中部夏令時間", "WIT": "印尼東部時間", "WESZ": "西歐夏令時間", "HNT": "紐芬蘭標準時間", "HAT": "紐芬蘭夏令時間", "ECT": "厄瓜多時間", "CHAST": "查坦群島標準時間", "MDT": "澳門夏令時間", "MYT": "馬來西亞時間", "VET": "委內瑞拉時間", "JDT": "日本夏令時間", "GMT": "格林威治標準時間", "UYST": "烏拉圭夏令時間", "HNNOMX": "墨西哥西北部標準時間", "OESZ": "東歐夏令時間", "WAST": "西非夏令時間", "HAST": "夏威夷-阿留申標準時間", "HADT": "夏威夷-阿留申夏令時間", "MESZ": "中歐夏令時間", "IST": "印度標準時間", "ACWST": "澳洲中西部標準時間", "NZDT": "紐西蘭夏令時間", "ACST": "澳洲中部標準時間", "CAT": "中非時間", "PDT": "太平洋夏令時間", "NZST": "紐西蘭標準時間", "LHST": "豪勳爵島標準時間"},
	}
}

// Locale returns the current translators string locale
func (zh *zh_Hant) Locale() string {
	return zh.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'zh_Hant'
func (zh *zh_Hant) PluralsCardinal() []locales.PluralRule {
	return zh.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'zh_Hant'
func (zh *zh_Hant) PluralsOrdinal() []locales.PluralRule {
	return zh.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'zh_Hant'
func (zh *zh_Hant) PluralsRange() []locales.PluralRule {
	return zh.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'zh_Hant'
func (zh *zh_Hant) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'zh_Hant'
func (zh *zh_Hant) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'zh_Hant'
func (zh *zh_Hant) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (zh *zh_Hant) MonthAbbreviated(month time.Month) string {
	return zh.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (zh *zh_Hant) MonthsAbbreviated() []string {
	return zh.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (zh *zh_Hant) MonthNarrow(month time.Month) string {
	return zh.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (zh *zh_Hant) MonthsNarrow() []string {
	return zh.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (zh *zh_Hant) MonthWide(month time.Month) string {
	return zh.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (zh *zh_Hant) MonthsWide() []string {
	return zh.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (zh *zh_Hant) WeekdayAbbreviated(weekday time.Weekday) string {
	return zh.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (zh *zh_Hant) WeekdaysAbbreviated() []string {
	return zh.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (zh *zh_Hant) WeekdayNarrow(weekday time.Weekday) string {
	return zh.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (zh *zh_Hant) WeekdaysNarrow() []string {
	return zh.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (zh *zh_Hant) WeekdayShort(weekday time.Weekday) string {
	return zh.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (zh *zh_Hant) WeekdaysShort() []string {
	return zh.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (zh *zh_Hant) WeekdayWide(weekday time.Weekday) string {
	return zh.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (zh *zh_Hant) WeekdaysWide() []string {
	return zh.daysWide
}

// Decimal returns the decimal point of number
func (zh *zh_Hant) Decimal() string {
	return zh.decimal
}

// Group returns the group of number
func (zh *zh_Hant) Group() string {
	return zh.group
}

// Group returns the minus sign of number
func (zh *zh_Hant) Minus() string {
	return zh.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'zh_Hant' and handles both Whole and Real numbers based on 'v'
func (zh *zh_Hant) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, zh.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, zh.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, zh.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'zh_Hant' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (zh *zh_Hant) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, zh.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, zh.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, zh.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'zh_Hant'
func (zh *zh_Hant) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := zh.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, zh.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, zh.group[0])
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
		b = append(b, zh.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, zh.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'zh_Hant'
// in accounting notation.
func (zh *zh_Hant) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := zh.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, zh.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, zh.group[0])
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

		b = append(b, zh.minus[0])

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
			b = append(b, zh.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0xe5, 0xb9, 0xb4}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0xe6, 0x9c, 0x88}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0xe6, 0x97, 0xa5}...)

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0xe5, 0xb9, 0xb4}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0xe6, 0x9c, 0x88}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0xe6, 0x97, 0xa5}...)

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0xe5, 0xb9, 0xb4}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0xe6, 0x9c, 0x88}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0xe6, 0x97, 0xa5, 0x20}...)
	b = append(b, zh.daysWide[t.Weekday()]...)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, zh.periodsAbbreviated[0]...)
	} else {
		b = append(b, zh.periodsAbbreviated[1]...)
	}

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, zh.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, zh.periodsAbbreviated[0]...)
	} else {
		b = append(b, zh.periodsAbbreviated[1]...)
	}

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, zh.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, zh.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, zh.periodsAbbreviated[0]...)
	} else {
		b = append(b, zh.periodsAbbreviated[1]...)
	}

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, zh.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, zh.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20, 0x5b}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	b = append(b, []byte{0x5d}...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'zh_Hant'
func (zh *zh_Hant) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 12 {
		b = append(b, zh.periodsAbbreviated[0]...)
	} else {
		b = append(b, zh.periodsAbbreviated[1]...)
	}

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, zh.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, zh.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20, 0x5b}...)

	tz, _ := t.Zone()

	if btz, ok := zh.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	b = append(b, []byte{0x5d}...)

	return string(b)
}
