package zh_Hans_CN

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type zh_Hans_CN struct {
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

// New returns a new instance of translator for the 'zh_Hans_CN' locale
func New() locales.Translator {
	return &zh_Hans_CN{
		locale:             "zh_Hans_CN",
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
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
		monthsNarrow:       []string{"", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
		monthsWide:         []string{"", "一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"},
		daysAbbreviated:    []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"},
		daysNarrow:         []string{"日", "一", "二", "三", "四", "五", "六"},
		daysShort:          []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"},
		daysWide:           []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"},
		periodsAbbreviated: []string{"上午", "下午"},
		periodsNarrow:      []string{"上午", "下午"},
		periodsWide:        []string{"上午", "下午"},
		erasAbbreviated:    []string{"公元前", "公元"},
		erasNarrow:         []string{"公元前", "公元"},
		erasWide:           []string{"公元前", "公元"},
		timezones:          map[string]string{"UYT": "乌拉圭标准时间", "TMT": "土库曼斯坦标准时间", "HNNOMX": "墨西哥西北部标准时间", "WIB": "印度尼西亚西部时间", "CDT": "北美中部夏令时间", "AWST": "澳大利亚西部标准时间", "WIT": "印度尼西亚东部时间", "AEST": "澳大利亚东部标准时间", "ECT": "厄瓜多尔标准时间", "MST": "澳门标准时间", "MESZ": "中欧夏令时间", "HAST": "夏威夷-阿留申标准时间", "JST": "日本标准时间", "IST": "印度时间", "ARST": "阿根廷夏令时间", "WAST": "西部非洲夏令时间", "HEEG": "格陵兰岛东部夏令时间", "CHAST": "查坦标准时间", "WESZ": "西欧夏令时间", "NZST": "新西兰标准时间", "OEZ": "东欧标准时间", "PDT": "北美太平洋夏令时间", "LHDT": "豪勋爵岛夏令时间", "HENOMX": "墨西哥西北部夏令时间", "HEOG": "格陵兰岛西部夏令时间", "HNEG": "格陵兰岛东部标准时间", "COT": "哥伦比亚标准时间", "HNPMX": "墨西哥太平洋标准时间", "AWDT": "澳大利亚西部夏令时间", "HNPM": "圣皮埃尔和密克隆群岛标准时间", "BT": "不丹时间", "CST": "北美中部标准时间", "ACWST": "澳大利亚中西部标准时间", "WARST": "阿根廷西部夏令时间", "ADT": "大西洋夏令时间", "HNOG": "格陵兰岛西部标准时间", "HECU": "古巴夏令时间", "HKST": "香港夏令时间", "COST": "哥伦比亚夏令时间", "CAT": "中部非洲时间", "HNT": "纽芬兰标准时间", "AEDT": "澳大利亚东部夏令时间", "AKST": "阿拉斯加标准时间", "CHADT": "查坦夏令时间", "WAT": "西部非洲标准时间", "CLT": "智利标准时间", "ACST": "澳大利亚中部标准时间", "HADT": "夏威夷-阿留申夏令时间", "EST": "北美东部标准时间", "VET": "委内瑞拉时间", "NZDT": "新西兰夏令时间", "UYST": "乌拉圭夏令时间", "∅∅∅": "巴西利亚夏令时间", "BOT": "玻利维亚标准时间", "SRT": "苏里南时间", "MYT": "马来西亚时间", "TMST": "土库曼斯坦夏令时间", "OESZ": "东欧夏令时间", "LHST": "豪勋爵岛标准时间", "HEPM": "圣皮埃尔和密克隆群岛夏令时间", "GFT": "法属圭亚那标准时间", "EDT": "北美东部夏令时间", "ChST": "查莫罗时间", "SAST": "南部非洲时间", "CLST": "智利夏令时间", "WEZ": "西欧标准时间", "SGT": "新加坡标准时间", "HEPMX": "墨西哥太平洋夏令时间", "ACWDT": "澳大利亚中西部夏令时间", "MDT": "澳门夏令时间", "WART": "阿根廷西部标准时间", "ART": "阿根廷标准时间", "EAT": "东部非洲时间", "HNCU": "古巴标准时间", "HAT": "纽芬兰夏令时间", "AST": "大西洋标准时间", "JDT": "日本夏令时间", "PST": "北美太平洋标准时间", "WITA": "印度尼西亚中部时间", "HKT": "香港标准时间", "GYT": "圭亚那时间", "AKDT": "阿拉斯加夏令时间", "ACDT": "澳大利亚中部夏令时间", "GMT": "格林尼治标准时间", "MEZ": "中欧标准时间"},
	}
}

// Locale returns the current translators string locale
func (zh *zh_Hans_CN) Locale() string {
	return zh.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'zh_Hans_CN'
func (zh *zh_Hans_CN) PluralsCardinal() []locales.PluralRule {
	return zh.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'zh_Hans_CN'
func (zh *zh_Hans_CN) PluralsOrdinal() []locales.PluralRule {
	return zh.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'zh_Hans_CN'
func (zh *zh_Hans_CN) PluralsRange() []locales.PluralRule {
	return zh.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (zh *zh_Hans_CN) MonthAbbreviated(month time.Month) string {
	return zh.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (zh *zh_Hans_CN) MonthsAbbreviated() []string {
	return zh.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (zh *zh_Hans_CN) MonthNarrow(month time.Month) string {
	return zh.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (zh *zh_Hans_CN) MonthsNarrow() []string {
	return zh.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (zh *zh_Hans_CN) MonthWide(month time.Month) string {
	return zh.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (zh *zh_Hans_CN) MonthsWide() []string {
	return zh.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (zh *zh_Hans_CN) WeekdayAbbreviated(weekday time.Weekday) string {
	return zh.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (zh *zh_Hans_CN) WeekdaysAbbreviated() []string {
	return zh.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (zh *zh_Hans_CN) WeekdayNarrow(weekday time.Weekday) string {
	return zh.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (zh *zh_Hans_CN) WeekdaysNarrow() []string {
	return zh.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (zh *zh_Hans_CN) WeekdayShort(weekday time.Weekday) string {
	return zh.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (zh *zh_Hans_CN) WeekdaysShort() []string {
	return zh.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (zh *zh_Hans_CN) WeekdayWide(weekday time.Weekday) string {
	return zh.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (zh *zh_Hans_CN) WeekdaysWide() []string {
	return zh.daysWide
}

// Decimal returns the decimal point of number
func (zh *zh_Hans_CN) Decimal() string {
	return zh.decimal
}

// Group returns the group of number
func (zh *zh_Hans_CN) Group() string {
	return zh.group
}

// Group returns the minus sign of number
func (zh *zh_Hans_CN) Minus() string {
	return zh.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'zh_Hans_CN' and handles both Whole and Real numbers based on 'v'
func (zh *zh_Hans_CN) FmtNumber(num float64, v uint64) string {

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

// FmtPercent returns 'num' with digits/precision of 'v' for 'zh_Hans_CN' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (zh *zh_Hans_CN) FmtPercent(num float64, v uint64) string {
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

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtCurrency(num float64, v uint64, currency currency.Type) string {

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

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'zh_Hans_CN'
// in accounting notation.
func (zh *zh_Hans_CN) FmtAccounting(num float64, v uint64, currency currency.Type) string {

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

// FmtDateShort returns the short date representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtDateMedium(t time.Time) string {

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

// FmtDateLong returns the long date representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtDateLong(t time.Time) string {

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

// FmtDateFull returns the full date representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtDateFull(t time.Time) string {

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
	b = append(b, zh.daysWide[t.Weekday()]...)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtTimeShort(t time.Time) string {

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

// FmtTimeMedium returns the medium time representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtTimeMedium(t time.Time) string {

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

// FmtTimeLong returns the long time representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	tz, _ := t.Zone()
	b = append(b, tz...)

	b = append(b, []byte{0x20}...)

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

// FmtTimeFull returns the full time representation of 't' for 'zh_Hans_CN'
func (zh *zh_Hans_CN) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	tz, _ := t.Zone()

	if btz, ok := zh.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	b = append(b, []byte{0x20}...)

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
