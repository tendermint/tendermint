package bn

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type bn struct {
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

// New returns a new instance of translator for the 'bn' locale
func New() locales.Translator {
	return &bn{
		locale:             "bn",
		pluralsCardinal:    []locales.PluralRule{2, 6},
		pluralsOrdinal:     []locales.PluralRule{2, 3, 4, 5, 6},
		pluralsRange:       []locales.PluralRule{2, 6},
		decimal:            ".",
		percent:            "%",
		perMille:           "‰",
		timeSeparator:      ":",
		inifinity:          "∞",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "A$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "৳", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JP¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "জানু", "ফেব", "মার্চ", "এপ্রিল", "মে", "জুন", "জুলাই", "আগস্ট", "সেপ্টেম্বর", "অক্টোবর", "নভেম্বর", "ডিসেম্বর"},
		monthsNarrow:       []string{"", "জা", "ফে", "মা", "এ", "মে", "জুন", "জু", "আ", "সে", "অ", "ন", "ডি"},
		monthsWide:         []string{"", "জানুয়ারী", "ফেব্রুয়ারী", "মার্চ", "এপ্রিল", "মে", "জুন", "জুলাই", "আগস্ট", "সেপ্টেম্বর", "অক্টোবর", "নভেম্বর", "ডিসেম্বর"},
		daysAbbreviated:    []string{"রবি", "সোম", "মঙ্গল", "বুধ", "বৃহস্পতি", "শুক্র", "শনি"},
		daysNarrow:         []string{"র", "সো", "ম", "বু", "বৃ", "শু", "শ"},
		daysShort:          []string{"রঃ", "সোঃ", "মঃ", "বুঃ", "বৃঃ", "শুঃ", "শোঃ"},
		daysWide:           []string{"রবিবার", "সোমবার", "মঙ্গলবার", "বুধবার", "বৃহস্পতিবার", "শুক্রবার", "শনিবার"},
		periodsAbbreviated: []string{"AM", "PM"},
		periodsNarrow:      []string{"AM", "PM"},
		periodsWide:        []string{"AM", "PM"},
		erasAbbreviated:    []string{"খ্রিস্টপূর্ব", "খৃষ্টাব্দ"},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"খ্রিস্টপূর্ব", "খৃষ্টাব্দ"},
		timezones:          map[string]string{"PDT": "প্রশান্ত মহাসাগরীয় অঞ্চলের দিনের সময়", "LHDT": "লর্ড হাওয়ে দিবালোক মসয়", "SAST": "দক্ষিণ আফ্রিকা মানক সময়", "AWST": "অস্ট্রেলীয় পশ্চিমি মানক সময়", "AWDT": "অস্ট্রেলীয় পশ্চিমি দিবালোক সময়", "HAST": "হাওয়াই-আলেউত মানক সময়", "OEZ": "পূর্ব ইউরোপের মানক সময়", "AEST": "অস্ট্রেলীয় পূর্ব মানক সময়", "ACST": "অস্ট্রেলীয় কেন্দ্রীয় মানক সময়", "HEPM": "সেন্ট পিয়ের ও মিকেলন দিবালোক সময়", "CST": "কেন্দ্রীয় মানক সময়", "UYT": "উরুগুয়ে মানক সময়", "WIT": "পূর্ব ইন্দোনেশিয়া সময়", "CHAST": "চ্যাথাম মানক সময়", "NZST": "নিউজিল্যান্ড মানক সময়", "AEDT": "অস্ট্রেলীয় পূর্ব দিবালোক সময়", "AKDT": "আলাস্কা দিবালোক সময়", "ART": "আর্জেনটিনা মানক সময়", "ARST": "আর্জেনটিনা গ্রীষ্মকালীন সময়", "EAT": "পূর্ব আফ্রিকা সময়", "HKST": "হং কং গ্রীষ্মকালীন সময়", "COST": "কোলোম্বিয়া গ্রীষ্মকালীন সময়", "HNPM": "সেন্ট পিয়ের ও মিকেলন মানক সময়", "MYT": "মালয়েশিয়া সময়", "NZDT": "নিউজিল্যান্ড দিবালোক সময়", "JST": "জাপান মানক সময়", "AST": "অতলান্তিক মানক সময়", "COT": "কোলোম্বিয়া মানক সময়", "HNNOMX": "উত্তরপশ্চিম মেক্সিকোর মানক সময়", "IST": "ভারতীয় মানক সময়", "WEZ": "পশ্চিম ইউরোপের মানক সময়", "WESZ": "পশ্চিম ইউরোপের গ্রীষ্মকালীন সময়", "HNPMX": "মেক্সিকান প্রশান্ত মহসাগরীয় মানক সময়", "HECU": "কিউবা দিবালোক সময়", "BT": "ভুটান সময়", "HADT": "হাওয়াই-আলেউত দিবালোক সময়", "JDT": "জাপান দিবালোক সময়", "WAT": "পশ্চিম আফ্রিকা মানক সময়", "ACDT": "অস্ট্রেলীয় কেন্দ্রীয় দিবালোক সময়", "∅∅∅": "অ্যামাজন গ্রীষ্মকালীন সময়", "CDT": "কেন্দ্রীয় দিবালোক সময়", "SRT": "সুরিনাম সময়", "ACWDT": "অস্ট্রেলীয় কেন্দ্রীয় পশ্চিমি দিবালোক সময়", "MEZ": "মধ্য ইউরোপের মানক সময়", "HNEG": "পূর্ব গ্রীনল্যান্ড মানক সময়", "HAT": "নিউফাউন্ডল্যান্ড দিবালোক সময়", "CLST": "চিলি গ্রীষ্মকাল সময়", "GYT": "গুয়ানা সময়", "ECT": "ইকুয়েডর সময়", "CHADT": "চ্যাথাম দিবালোক সময়", "HNCU": "কিউবা মানক সময়", "MESZ": "মধ্য ইউরোপের গ্রীষ্মকালীন সময়", "WARST": "পশ্চিমি আর্জেনটিনা গৃষ্মকালীন সময়", "GFT": "ফরাসি গায়ানা সময়", "CLT": "চিলি মানক সময়", "PST": "প্রশান্ত মহাসাগরীয় অঞ্চলের মানক সময়", "BOT": "বোলিভিয়া সময়", "MST": "মাকাও মান সময়", "ACWST": "অস্ট্রেলীয় কেন্দ্রীয় পশ্চিমি মানক সময়", "TMT": "তুর্কমেনিস্তান মানক সময়", "VET": "ভেনেজুয়েলা সময়", "ChST": "চামেরো মানক সময়", "WITA": "কেন্দ্রীয় ইন্দোনেশিয়া সময়", "EDT": "পূর্বাঞ্চলের দিবালোক সময়", "CAT": "মধ্য আফ্রিকা সময়", "HEPMX": "মেক্সিকান প্রশান্ত মহাসাগরীয় দিবালোক সময়", "OESZ": "পূর্ব ইউরোপের গ্রীষ্মকালীন সময়", "LHST": "লর্ড হাওয়ে মানক মসয়", "HEEG": "পূর্ব গ্রীনল্যান্ড গ্রীষ্মকালীন সময়", "HNT": "নিউফাউন্ডল্যান্ড মানক সময়", "AKST": "আলাস্কা মানক সময়", "MDT": "মাকাও গ্রীষ্মকাল সময়", "ADT": "অতলান্তিক দিবালোক সময়", "SGT": "সিঙ্গাপুর মানক সময়", "GMT": "গ্রীনিচ মিন টাইম", "HNOG": "পশ্চিম গ্রীনল্যান্ড মানক সময়", "EST": "পূর্বাঞ্চলের প্রমাণ সময়", "TMST": "তুর্কমেনিস্তান গ্রীষ্মকালীন সময়", "WART": "পশ্চিমি আর্জেনটিনার প্রমাণ সময়", "WAST": "পশ্চিম আফ্রিকা গ্রীষ্মকালীন সময়", "WIB": "পশ্চিমী ইন্দোনেশিয়া সময়", "UYST": "উরুগুয়ে গ্রীষ্মকালীন সময়", "HENOMX": "উত্তরপশ্চিম মেক্সিকোর দিনের সময়", "HEOG": "পশ্চিম গ্রীনল্যান্ড গ্রীষ্মকালীন সময়", "HKT": "হং কং মানক সময়"},
	}
}

// Locale returns the current translators string locale
func (bn *bn) Locale() string {
	return bn.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'bn'
func (bn *bn) PluralsCardinal() []locales.PluralRule {
	return bn.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'bn'
func (bn *bn) PluralsOrdinal() []locales.PluralRule {
	return bn.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'bn'
func (bn *bn) PluralsRange() []locales.PluralRule {
	return bn.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'bn'
func (bn *bn) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if (i == 0) || (n == 1) {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'bn'
func (bn *bn) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 || n == 5 || n == 7 || n == 8 || n == 9 || n == 10 {
		return locales.PluralRuleOne
	} else if n == 2 || n == 3 {
		return locales.PluralRuleTwo
	} else if n == 4 {
		return locales.PluralRuleFew
	} else if n == 6 {
		return locales.PluralRuleMany
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'bn'
func (bn *bn) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := bn.CardinalPluralRule(num1, v1)
	end := bn.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOne {
		return locales.PluralRuleOne
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (bn *bn) MonthAbbreviated(month time.Month) string {
	return bn.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (bn *bn) MonthsAbbreviated() []string {
	return bn.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (bn *bn) MonthNarrow(month time.Month) string {
	return bn.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (bn *bn) MonthsNarrow() []string {
	return bn.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (bn *bn) MonthWide(month time.Month) string {
	return bn.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (bn *bn) MonthsWide() []string {
	return bn.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (bn *bn) WeekdayAbbreviated(weekday time.Weekday) string {
	return bn.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (bn *bn) WeekdaysAbbreviated() []string {
	return bn.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (bn *bn) WeekdayNarrow(weekday time.Weekday) string {
	return bn.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (bn *bn) WeekdaysNarrow() []string {
	return bn.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (bn *bn) WeekdayShort(weekday time.Weekday) string {
	return bn.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (bn *bn) WeekdaysShort() []string {
	return bn.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (bn *bn) WeekdayWide(weekday time.Weekday) string {
	return bn.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (bn *bn) WeekdaysWide() []string {
	return bn.daysWide
}

// Decimal returns the decimal point of number
func (bn *bn) Decimal() string {
	return bn.decimal
}

// Group returns the group of number
func (bn *bn) Group() string {
	return bn.group
}

// Group returns the minus sign of number
func (bn *bn) Minus() string {
	return bn.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'bn' and handles both Whole and Real numbers based on 'v'
func (bn *bn) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 1
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, bn.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, bn.group[0])
				count = 1

				if !inSecondary {
					inSecondary = true
					groupThreshold = 2
				}
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, bn.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'bn' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (bn *bn) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, bn.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, bn.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, bn.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'bn'
func (bn *bn) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := bn.currencies[currency]
	l := len(s) + len(symbol) + 1
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, bn.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, bn.group[0])
				count = 1

				if !inSecondary {
					inSecondary = true
					groupThreshold = 2
				}
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, bn.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, bn.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'bn'
// in accounting notation.
func (bn *bn) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := bn.currencies[currency]
	l := len(s) + len(symbol) + 1
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, bn.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, bn.group[0])
				count = 1

				if !inSecondary {
					inSecondary = true
					groupThreshold = 2
				}
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, bn.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, bn.decimal...)
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

// FmtDateShort returns the short date representation of 't' for 'bn'
func (bn *bn) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'bn'
func (bn *bn) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, bn.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'bn'
func (bn *bn) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, bn.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'bn'
func (bn *bn) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, bn.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, bn.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'bn'
func (bn *bn) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, bn.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, bn.periodsAbbreviated[0]...)
	} else {
		b = append(b, bn.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'bn'
func (bn *bn) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, bn.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, bn.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, bn.periodsAbbreviated[0]...)
	} else {
		b = append(b, bn.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'bn'
func (bn *bn) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, bn.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, bn.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, bn.periodsAbbreviated[0]...)
	} else {
		b = append(b, bn.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'bn'
func (bn *bn) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, bn.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, bn.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, bn.periodsAbbreviated[0]...)
	} else {
		b = append(b, bn.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := bn.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
