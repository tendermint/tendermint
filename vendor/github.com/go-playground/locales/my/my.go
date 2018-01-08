package my

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type my struct {
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
	currencyNegativePrefix string
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

// New returns a new instance of translator for the 'my' locale
func New() locales.Translator {
	return &my{
		locale:                 "my",
		pluralsCardinal:        []locales.PluralRule{6},
		pluralsOrdinal:         []locales.PluralRule{6},
		pluralsRange:           []locales.PluralRule{6},
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "NAf", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "A$", "Afl", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "G", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JP¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "K", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "B/.", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TT$", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositivePrefix: " ",
		currencyNegativePrefix: " ",
		monthsAbbreviated:      []string{"", "ဇန်", "ဖေ", "မတ်", "ဧ", "မေ", "ဇွန်", "ဇူ", "ဩ", "စက်", "အောက်", "နို", "ဒီ"},
		monthsNarrow:           []string{"", "ဇ", "ဖ", "မ", "ဧ", "မ", "ဇ", "ဇ", "ဩ", "စ", "အ", "န", "ဒ"},
		monthsWide:             []string{"", "ဇန်နဝါရီ", "ဖေဖော်ဝါရီ", "မတ်", "ဧပြီ", "မေ", "ဇွန်", "ဇူလိုင်", "ဩဂုတ်", "စက်တင်ဘာ", "အောက်တိုဘာ", "နိုဝင်ဘာ", "ဒီဇင်ဘာ"},
		daysAbbreviated:        []string{"တနင်္ဂနွေ", "တနင်္လာ", "အင်္ဂါ", "ဗုဒ္ဓဟူး", "ကြာသပတေး", "သောကြာ", "စနေ"},
		daysNarrow:             []string{"တ", "တ", "အ", "ဗ", "က", "သ", "စ"},
		daysShort:              []string{"တနင်္ဂနွေ", "တနင်္လာ", "အင်္ဂါ", "ဗုဒ္ဓဟူး", "ကြာသပတေး", "သောကြာ", "စနေ"},
		daysWide:               []string{"တနင်္ဂနွေ", "တနင်္လာ", "အင်္ဂါ", "ဗုဒ္ဓဟူး", "ကြာသပတေး", "သောကြာ", "စနေ"},
		periodsAbbreviated:     []string{"နံနက်", "ညနေ"},
		periodsNarrow:          []string{"နံနက်", "ညနေ"},
		periodsWide:            []string{"နံနက်", "ညနေ"},
		erasAbbreviated:        []string{"ဘီစီ", "အေဒီ"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"ခရစ်တော် မပေါ်မီနှစ်", "ခရစ်နှစ်"},
		timezones:              map[string]string{"HAT": "နယူးဖောင်လန် နွေရာသီ စံတော်ချိန်", "ChST": "ချာမိုရိုအချိန်", "GMT": "ဂရင်းနစ် စံတော်ချိန်", "MESZ": "ဥရောပ အလယ်ပိုင်း နွေရာသီ အချိန်", "VET": "ဗင်နီဇွဲလား အချိန်", "ARST": "အာဂျင်တီးနား နွေရာသီအချိန်", "EAT": "အရှေ့အာဖရိက အချိန်", "SGT": "စင်္ကာပူ အချိန်", "HEPMX": "မက္ကစီကန် ပစိဖိတ် နွေရာသီ စံတော်ချိန်", "CST": "မြောက်အမေရိက အလယ်ပိုင်းစံတော်ချိန်", "AEDT": "အရှေ့ဩစတြေးလျား နွေရာသီ အချိန်", "AKDT": "အလားစကား နွေရာသီစံတော်ချိန်", "WEZ": "အနောက်ဥရောပ စံတော်ချိန်", "JDT": "ဂျပန် နွေရာသီ အချိန်", "SAST": "တောင်အာဖရိက အချိန်", "HNEG": "အရှေ့ဂရင်းလန်း စံတော်ချိန်", "HNPMX": "မက္ကဆီကန် ပစိဖိတ် စံတော်ချိန်", "CDT": "မြောက်အမေရိက အလယ်ပိုင်း နွေရာသီစံတော်ချိန်", "UYT": "ဥရုဂွေး စံတော်ချိန်", "ACWST": "ဩစတြေးလျား အလယ်အနောက်ပိုင်း စံတော်ချိန်", "HADT": "ဟာဝိုင်ယီ အယ်လူးရှန်း နွေရာသီ စံတော်ချိန်", "OEZ": "အရှေ့ဥရောပ စံတော်ချိန်", "AEST": "အရှေ့ဩစတြေးလျား စံတော်ချိန်", "CLT": "ချီလီ စံတော်ချိန်", "HNCU": "ကျူးဘား စံတော်ချိန်", "MYT": "မလေးရှား အချိန်", "WAT": "အနောက်အာဖရိက စံတော်ချိန်", "ACDT": "ဩစတြေးလျား အလယ်ပိုင်း နွေရာသီ အချိန်", "CAT": "အလယ်အာဖရိက အချိန်", "ACWDT": "ဩစတြေးလျား အလယ်အနောက်ပိုင်း နွေရာသီ အချိန်", "WARST": "အနောက် အာဂျင်တီးနား နွေရာသီ အချိန်", "HENOMX": "အနောက်တောင် မက္ကစီကို နွေရာသီ စံတော်ချိန်", "GFT": "ပြင်သစ် ဂီအားနား အချိန်", "WIB": "အနောက်ပိုင်း အင်ဒိုနီးရှား အချိန်", "AST": "အတ္တလန်တစ် စံတော်ချိန်", "CLST": "ချီလီ နွေရာသီ အချိန်", "COT": "ကိုလံဘီယာ စံတော်ချိန်", "HEPM": "စိန့်ပီအဲနှင့် မီခွီလွန် နွေရာသီ စံတော်ချိန်", "WIT": "အရှေ့ပိုင်း အင်ဒိုနီးရှား အချိန်", "TMST": "တာ့ခ်မင်နစ္စတန် နွေရာသီ အချိန်", "∅∅∅": "အေဇိုးရီးစ် နွေရာသီ အချိန်", "ADT": "အတ္တလန်တစ် နွေရာသီ စံတော်ချိန်", "HEEG": "အရှေ့ဂရင်းလန် နွေရာသီ စံတော်ချိန်", "HNT": "နယူးဖောင်လန် စံတော်ချိန်", "AKST": "အလားစကားစံတော်ချိန်", "AWDT": "ဩစတြေးလျား နွေရာသီ အချိန်", "UYST": "ဥရုဂွေး နွေရာသီ အချိန်", "JST": "ဂျပန် စံတော်ချိန်", "ART": "အာဂျင်တီးနား စံတော်ချိန်", "HKT": "ဟောင်ကောင် စံတော်ချိန်", "EST": "မြောက်အမေရိက အရှေ့ပိုင်းစံတော်ချိန်", "HECU": "ကျူးဘား နွေရာသီ စံတော်ချိန်", "BOT": "ဘိုလီးဘီးယား အချိန်", "MST": "မြောက်အမေရိက တောင်တန်းဒေသစံတော်ချိန်", "WITA": "အလယ်ပိုင်း အင်ဒိုနီးရှား အချိန်", "OESZ": "အရှေ့ဥရောပ နွေရာသီ အချိန်", "TMT": "တာ့ခ်မင်နစ္စတန် စံတော်ချိန်", "WART": "အနောက် အာဂျင်တီးနား စံတော်ချိန်", "COST": "ကိုလံဘီယာ နွေရာသီ အချိန်", "SRT": "စူးရီနာမ်အချိန်", "GYT": "ဂိုင်ယာနာ အချိန်", "CHAST": "ချားသမ်စံတော်ချိန်", "PDT": "မြောက်အမေရိက ပစိဖိတ် နွေရာသီစံတော်ချိန်", "MDT": "မြောက်အမေရိက တောင်တန်းဒေသ နွေရာသီစံတော်ချိန်", "IST": "အိန္ဒိယ စံတော်ချိန်", "HNOG": "အနောက် ဂရင်းလန်း စံတော်ချိန်", "HEOG": "အနောက် ဂရင်းလန် နွေရာသီ စံတော်ချိန်", "ECT": "အီကွေဒေါ အချိန်", "BT": "ဘူတန် အချိန်", "HAST": "ဟာဝိုင်ယီ အယ်လူးရှန်း စံတော်ချိန်", "NZST": "နယူးဇီလန် စံတော်ချိန်", "LHST": "လော့ဒ်ဟောင်စံတော်ချိန်", "LHDT": "လော့ဒ်ဟောင် နွေရာသီ အချိန်", "HNNOMX": "အနောက်တောင် မက္ကဆီကို စံတော်ချိန်", "NZDT": "နယူးဇီလန် နွေရာသီ အချိန်", "CHADT": "ချာသမ် နွေရာသီ အချိန်", "HNPM": "စိန့်ပီအဲနှင့်မီခွီလွန်စံတော်ချိန်", "MEZ": "ဥရောပ အလယ်ပိုင်း စံတော်ချိန်", "WAST": "အနောက်အာဖရိက နွေရာသီ အချိန်", "EDT": "မြောက်အမေရိက အရှေ့ပိုင်း နွေရာသီစံတော်ချိန်", "WESZ": "အနောက်ဥရောပ နွေရာသီ အချိန်", "AWST": "အနောက်ဩစတြေးလျား စံတော်ချိန်", "HKST": "ဟောင်ကောင် နွေရာသီ အချိန်", "ACST": "ဩစတြေးလျား အလယ်ပိုင်း စံတော်ချိန်", "PST": "မြောက်အမေရိက ပစိဖိတ်စံတော်ချိန်"},
	}
}

// Locale returns the current translators string locale
func (my *my) Locale() string {
	return my.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'my'
func (my *my) PluralsCardinal() []locales.PluralRule {
	return my.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'my'
func (my *my) PluralsOrdinal() []locales.PluralRule {
	return my.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'my'
func (my *my) PluralsRange() []locales.PluralRule {
	return my.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'my'
func (my *my) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'my'
func (my *my) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'my'
func (my *my) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (my *my) MonthAbbreviated(month time.Month) string {
	return my.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (my *my) MonthsAbbreviated() []string {
	return my.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (my *my) MonthNarrow(month time.Month) string {
	return my.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (my *my) MonthsNarrow() []string {
	return my.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (my *my) MonthWide(month time.Month) string {
	return my.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (my *my) MonthsWide() []string {
	return my.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (my *my) WeekdayAbbreviated(weekday time.Weekday) string {
	return my.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (my *my) WeekdaysAbbreviated() []string {
	return my.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (my *my) WeekdayNarrow(weekday time.Weekday) string {
	return my.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (my *my) WeekdaysNarrow() []string {
	return my.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (my *my) WeekdayShort(weekday time.Weekday) string {
	return my.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (my *my) WeekdaysShort() []string {
	return my.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (my *my) WeekdayWide(weekday time.Weekday) string {
	return my.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (my *my) WeekdaysWide() []string {
	return my.daysWide
}

// Decimal returns the decimal point of number
func (my *my) Decimal() string {
	return my.decimal
}

// Group returns the group of number
func (my *my) Group() string {
	return my.group
}

// Group returns the minus sign of number
func (my *my) Minus() string {
	return my.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'my' and handles both Whole and Real numbers based on 'v'
func (my *my) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 0
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, my.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, my.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, my.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'my' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (my *my) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 1
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, my.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, my.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, my.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'my'
func (my *my) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := my.currencies[currency]
	l := len(s) + len(symbol) + 2
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, my.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, my.group[0])
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

	for j := len(my.currencyPositivePrefix) - 1; j >= 0; j-- {
		b = append(b, my.currencyPositivePrefix[j])
	}

	if num < 0 {
		b = append(b, my.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, my.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'my'
// in accounting notation.
func (my *my) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := my.currencies[currency]
	l := len(s) + len(symbol) + 2
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, my.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, my.group[0])
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

		for j := len(my.currencyNegativePrefix) - 1; j >= 0; j-- {
			b = append(b, my.currencyNegativePrefix[j])
		}

		b = append(b, my.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(my.currencyPositivePrefix) - 1; j >= 0; j-- {
			b = append(b, my.currencyPositivePrefix[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, my.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'my'
func (my *my) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2d}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2d}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'my'
func (my *my) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, my.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'my'
func (my *my) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, my.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'my'
func (my *my) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, my.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, my.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'my'
func (my *my) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, my.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'my'
func (my *my) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, my.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, my.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'my'
func (my *my) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	tz, _ := t.Zone()
	b = append(b, tz...)

	b = append(b, []byte{0x20}...)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, my.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, my.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'my'
func (my *my) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	tz, _ := t.Zone()

	if btz, ok := my.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	b = append(b, []byte{0x20}...)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, my.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, my.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}
