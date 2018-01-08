package dz_BT

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type dz_BT struct {
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

// New returns a new instance of translator for the 'dz_BT' locale
func New() locales.Translator {
	return &dz_BT{
		locale:             "dz_BT",
		pluralsCardinal:    []locales.PluralRule{6},
		pluralsOrdinal:     nil,
		pluralsRange:       nil,
		decimal:            ".",
		group:              ",",
		minus:              "-",
		percent:            "%",
		perMille:           "‰",
		timeSeparator:      ":",
		inifinity:          "གྲངས་མེད",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:      " ",
		monthsAbbreviated:  []string{"", "༡", "༢", "༣", "༤", "༥", "༦", "༧", "༨", "༩", "༡༠", "༡༡", "12"},
		monthsNarrow:       []string{"", "༡", "༢", "༣", "4", "༥", "༦", "༧", "༨", "9", "༡༠", "༡༡", "༡༢"},
		monthsWide:         []string{"", "ཟླ་དངཔ་", "ཟླ་གཉིས་པ་", "ཟླ་གསུམ་པ་", "ཟླ་བཞི་པ་", "ཟླ་ལྔ་པ་", "ཟླ་དྲུག་པ", "ཟླ་བདུན་པ་", "ཟླ་བརྒྱད་པ་", "ཟླ་དགུ་པ་", "ཟླ་བཅུ་པ་", "ཟླ་བཅུ་གཅིག་པ་", "ཟླ་བཅུ་གཉིས་པ་"},
		daysAbbreviated:    []string{"ཟླ་", "མིར་", "ལྷག་", "ཕུར་", "སངས་", "སྤེན་", "ཉི་"},
		daysNarrow:         []string{"ཟླ", "མིར", "ལྷག", "ཕུར", "སངྶ", "སྤེན", "ཉི"},
		daysShort:          []string{"ཟླ་", "མིར་", "ལྷག་", "ཕུར་", "སངས་", "སྤེན་", "ཉི་"},
		daysWide:           []string{"གཟའ་ཟླ་བ་", "གཟའ་མིག་དམར་", "གཟའ་ལྷག་པ་", "གཟའ་ཕུར་བུ་", "གཟའ་པ་སངས་", "གཟའ་སྤེན་པ་", "གཟའ་ཉི་མ་"},
		periodsAbbreviated: []string{"སྔ་ཆ་", "ཕྱི་ཆ་"},
		periodsNarrow:      []string{"སྔ་ཆ་", "ཕྱི་ཆ་"},
		periodsWide:        []string{"སྔ་ཆ་", "ཕྱི་ཆ་"},
		erasAbbreviated:    []string{"BCE", "CE"},
		erasNarrow:         []string{"", ""},
		erasWide:           []string{"", ""},
		timezones:          map[string]string{"TMST": "TMST", "IST": "རྒྱ་གར་ཆུ་ཚོད", "SAST": "ལྷོ་ཕྱོགས་ཨཕ་རི་ཀཱ་ཆུ་ཚོད", "ACDT": "དབུས་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཉིན་སྲུང་ཆུ་ཚོད", "WEZ": "ནུབ་ཕྱོགས་ཡུ་རོ་པེན་ཚད་ལྡན་ཆུ་ཚོད", "HNPMX": "HNPMX", "MST": "MST", "WAST": "ནུབ་ཕྱོགས་ཨཕ་རི་ཀཱ་བྱཱར་དུས་ཆུ་ཚོད", "ART": "ཨར་ཇེན་ཊི་ན་ཚད་ལྡན་ཆུ་ཚོད", "ARST": "ཨར་ཇེན་ཊི་ན་བྱཱར་དུས་ཆུ་ཚོད", "HEOG": "ནུབ་ཕྱོགས་གིརིན་ལེནཌ་བྱཱར་དུས་ཆུ་ཚོད", "HECU": "ཀིའུ་བྷ་ཉིན་སྲུང་ཆུ་ཚོད", "CDT": "བྱང་ཨ་མི་རི་ཀ་དབུས་ཕྱོགས་ཉིན་སྲུང་ཆུ་ཚོད", "SRT": "སུ་རི་ནཱམ་ཆུ་ཚོད", "ACWDT": "དབུས་ནུབ་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཉིན་སྲུང་ཆུ་ཚོད", "WIT": "ཤར་ཕྱོགས་ཨིན་ཌོ་ནེ་ཤི་ཡ་ཆུ་ཚོད", "NZST": "ནིའུ་ཛི་ལེནཌ་ཚད་ལྡན་ཆུ་ཚོད", "HENOMX": "HENOMX", "LHDT": "LHDT", "AEDT": "ཤར་ཕྱོགས་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཉིན་སྲུང་ཆུ་ཚོད", "∅∅∅": "∅∅∅", "WESZ": "ནུབ་ཕྱོགས་ཡུ་རོ་པེན་བྱཱར་དུས་ཆུ་ཚོད", "HEPMX": "HEPMX", "ECT": "ཨེ་ཀུ་ཌཽ་ཆུ་ཚོད", "HNEG": "ཤར་ཕྱོགས་གིརིན་ལེནཌ་ཚད་ལྡན་ཆུ་ཚོད", "CAT": "དབུས་ཕྱོགས་ཨཕ་རི་ཀཱ་ཆུ་ཚོད", "GMT": "གིརིན་ཝིཆ་ལུ་ཡོད་པའི་ཆུ་ཚོད", "BT": "འབྲུག་ཡུལ་ཆུ་ཚོད", "VET": "བེ་ནི་ཛུ་ཝེ་ལ་ཆུ་ཚོད", "AWST": "ནུབ་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཚད་ལྡན་ཆུ་ཚོད", "HNNOMX": "HNNOMX", "WITA": "དབུས་ཕྱོགས་ཨིན་ཌོ་ནེ་ཤི་ཡ་ཆུ་ཚོད", "HNOG": "ནུབ་ཕྱོགས་གིརིན་ལེནཌ་ཚད་ལྡན་ཆུ་ཚོད", "EDT": "བྱང་ཨ་མི་རི་ཀ་ཤར་ཕྱོགས་ཉིན་སྲུང་ཆུ་ཚོད", "GYT": "གུ་ཡ་ན་ཆུ་ཚོད", "AKST": "ཨ་ལསི་ཀ་ཚད་ལྡན་ཆུ་ཚོད", "CST": "བྱང་ཨ་མི་རི་ཀ་དབུས་ཕྱོགས་ཚད་ལྡན་ཆུ་ཚོད", "HAST": "ཧ་ཝའི་-ཨེ་ལིའུ་ཤེན་ཚད་ལྡན་ཆུ་ཚོད", "HEPM": "པའི་རི་དང་མི་ཀི་ལཱོན་ཉིན་སྲུང་ཆུ་ཚོད", "UYT": "ཡུ་རུ་གུ་ཝཱའི་ཚད་ལྡན་ཆུ་ཚོད", "LHST": "LHST", "WART": "ནུབ་ཕྱོགས་ཨར་ཇེན་ཊི་ན་ཚད་ལྡན་ཆུ་ཚོད", "COT": "ཀོ་ལོམ་བྷི་ཡ་ཚད་ལྡན་ཆུ་ཚོད", "WIB": "ནུབ་ཕྱོགས་ཨིན་ཌོ་ནེ་ཤི་ཡ་ཆུ་ཚོད", "CHAST": "CHAST", "OESZ": "ཤར་ཕྱོགས་ཡུ་རོ་པེན་བྱཱར་དུས་ཆུ་ཚོད", "UYST": "ཡུ་རུ་གུ་ཝཱའི་བྱཱར་དུས་ཆུ་ཚོད", "JST": "ཇ་པཱན་ཚད་ལྡན་ཆུ་ཚོད", "HNT": "ནིའུ་ཕའུནཌ་ལེནཌ་ཚད་ལྡན་ཆུ་ཚོད", "ACST": "དབུས་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཚད་ལྡན་ཆུ་ཚོད", "PST": "བྱང་ཨ་མི་རི་ཀ་པེ་སི་ཕིག་ཚད་ལྡན་ཆུ་ཚོད", "PDT": "བྱང་ཨ་མི་རི་ཀ་པེ་སི་ཕིག་ཉིན་སྲུང་ཆུ་ཚོད", "MDT": "MDT", "AEST": "ཤར་ཕྱོགས་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཚད་ལྡན་ཆུ་ཚོད", "AKDT": "ཨ་ལསི་ཀ་ཉིན་སྲུང་ཆུ་ཚོད", "BOT": "བྷོ་ལི་བི་ཡ་ཆུ་ཚོད", "ADT": "ཨེཊ་ལེན་ཊིཀ་ཉིན་སྲུང་ཆུ་ཚོད", "HNPM": "པའི་རི་དང་མི་ཀི་ལཱོན་ཚད་ལྡན་ཆུ་ཚོད", "EAT": "ཤར་ཕྱོགས་ཨཕ་རི་ཀཱ་ཆུ་ཚོད", "EST": "བྱང་ཨ་མི་རི་ཀ་ཤར་ཕྱོགས་ཚད་ལྡན་ཆུ་ཚོད", "ChST": "ChST", "HNCU": "ཀིའུ་བྷ་ཚད་ལྡན་ཆུ་ཚོད", "AWDT": "ནུབ་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཉིན་སྲུང་ཆུ་ཚོད", "AST": "ཨེཊ་ལེན་ཊིཀ་ཚད་ལྡན་ཆུ་ཚོད", "WAT": "ནུབ་ཕྱོགས་ཨཕ་རི་ཀཱ་ཚད་ལྡན་ཆུ་ཚོད", "HEEG": "ཤར་ཕྱོགས་གིརིན་ལེནཌ་བྱཱར་དུས་ཆུ་ཚོད", "HKST": "HKST", "OEZ": "ཤར་ཕྱོགས་ཡུ་རོ་པེན་ཚད་ལྡན་ཆུ་ཚོད", "NZDT": "ནིའུ་ཛི་ལེནཌ་ཉིན་སྲུང་ཆུ་ཚོད", "CLST": "ཅི་ལི་བྱཱར་དུས་ཆུ་ཚོད", "COST": "ཀོ་ལོམ་བྷི་ཡ་བྱཱར་དུས་ཆུ་ཚོད", "MEZ": "དབུས་ཕྱོགས་ཡུ་རོ་པེན་ཚད་ལྡན་ཆུ་ཚོད", "HADT": "ཧ་ཝའི་-ཨེ་ལིའུ་ཤེན་ཉིན་སྲུང་ཆུ་ཚོད", "TMT": "TMT", "HAT": "ནིའུ་ཕའུནཌ་ལེནཌ་ཉིན་སྲུང་ཆུ་ཚོད", "SGT": "SGT", "MESZ": "དབུས་ཕྱོགས་ཡུ་རོ་པེན་བྱཱར་དུས་ཆུ་ཚོད", "MYT": "MYT", "JDT": "ཇ་པཱན་ཉིན་སྲུང་ཆུ་ཚོད", "WARST": "ནུབ་ཕྱོགས་ཨར་ཇེན་ཊི་ན་བྱཱར་དུས་ཆུ་ཚོད", "GFT": "ཕིརེནཅ་གི་ཡ་ན་ཆུ་ཚོད", "HKT": "HKT", "CLT": "ཅི་ལི་ཚད་ལྡན་ཆུ་ཚོད", "CHADT": "CHADT", "ACWST": "དབུས་ནུབ་ཕྱོགས་ཨཱོས་ཊྲེལ་ལི་ཡ་ཚད་ལྡན་ཆུ་ཚོད"},
	}
}

// Locale returns the current translators string locale
func (dz *dz_BT) Locale() string {
	return dz.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'dz_BT'
func (dz *dz_BT) PluralsCardinal() []locales.PluralRule {
	return dz.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'dz_BT'
func (dz *dz_BT) PluralsOrdinal() []locales.PluralRule {
	return dz.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'dz_BT'
func (dz *dz_BT) PluralsRange() []locales.PluralRule {
	return dz.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'dz_BT'
func (dz *dz_BT) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'dz_BT'
func (dz *dz_BT) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'dz_BT'
func (dz *dz_BT) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (dz *dz_BT) MonthAbbreviated(month time.Month) string {
	return dz.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (dz *dz_BT) MonthsAbbreviated() []string {
	return dz.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (dz *dz_BT) MonthNarrow(month time.Month) string {
	return dz.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (dz *dz_BT) MonthsNarrow() []string {
	return dz.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (dz *dz_BT) MonthWide(month time.Month) string {
	return dz.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (dz *dz_BT) MonthsWide() []string {
	return dz.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (dz *dz_BT) WeekdayAbbreviated(weekday time.Weekday) string {
	return dz.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (dz *dz_BT) WeekdaysAbbreviated() []string {
	return dz.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (dz *dz_BT) WeekdayNarrow(weekday time.Weekday) string {
	return dz.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (dz *dz_BT) WeekdaysNarrow() []string {
	return dz.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (dz *dz_BT) WeekdayShort(weekday time.Weekday) string {
	return dz.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (dz *dz_BT) WeekdaysShort() []string {
	return dz.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (dz *dz_BT) WeekdayWide(weekday time.Weekday) string {
	return dz.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (dz *dz_BT) WeekdaysWide() []string {
	return dz.daysWide
}

// Decimal returns the decimal point of number
func (dz *dz_BT) Decimal() string {
	return dz.decimal
}

// Group returns the group of number
func (dz *dz_BT) Group() string {
	return dz.group
}

// Group returns the minus sign of number
func (dz *dz_BT) Minus() string {
	return dz.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'dz_BT' and handles both Whole and Real numbers based on 'v'
func (dz *dz_BT) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, dz.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, dz.group[0])
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
		b = append(b, dz.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'dz_BT' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (dz *dz_BT) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, dz.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, dz.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, dz.percentSuffix...)

	b = append(b, dz.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'dz_BT'
func (dz *dz_BT) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := dz.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, dz.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, dz.group[0])
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

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	if num < 0 {
		b = append(b, dz.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, dz.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'dz_BT'
// in accounting notation.
func (dz *dz_BT) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := dz.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, dz.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, dz.group[0])
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

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		b = append(b, dz.minus[0])

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
			b = append(b, dz.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x2d}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2d}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0xe0, 0xbd, 0xa6, 0xe0, 0xbe, 0xa4, 0xe0, 0xbe, 0xb1, 0xe0, 0xbd, 0xb2, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0xa3, 0xe0, 0xbd, 0xbc, 0xe0, 0xbc, 0x8b}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20, 0xe0, 0xbd, 0x9f, 0xe0, 0xbe, 0xb3, 0xe0, 0xbc, 0x8b}...)
	b = append(b, dz.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xba, 0xe0, 0xbd, 0xa6, 0xe0, 0xbc, 0x8b}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0xe0, 0xbd, 0xa6, 0xe0, 0xbe, 0xa4, 0xe0, 0xbe, 0xb1, 0xe0, 0xbd, 0xb2, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0xa3, 0xe0, 0xbd, 0xbc, 0xe0, 0xbc, 0x8b}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, dz.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xba, 0xe0, 0xbd, 0xa6, 0xe0, 0xbc, 0x8b, 0x20}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, dz.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20, 0xe0, 0xbd, 0xa6, 0xe0, 0xbe, 0xa4, 0xe0, 0xbe, 0xb1, 0xe0, 0xbd, 0xb2, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0xa3, 0xe0, 0xbd, 0xbc, 0xe0, 0xbc, 0x8b}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, dz.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xba, 0xe0, 0xbd, 0xa6, 0xe0, 0xbc, 0x8b}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0xe0, 0xbd, 0x86, 0xe0, 0xbd, 0xb4, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xbc, 0xe0, 0xbd, 0x91, 0xe0, 0xbc, 0x8b, 0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, []byte{0x20, 0xe0, 0xbd, 0xa6, 0xe0, 0xbe, 0x90, 0xe0, 0xbd, 0xa2, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x98, 0xe0, 0xbc, 0x8b, 0x20}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, dz.periodsAbbreviated[0]...)
	} else {
		b = append(b, dz.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0xe0, 0xbd, 0x86, 0xe0, 0xbd, 0xb4, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xbc, 0xe0, 0xbd, 0x91, 0xe0, 0xbc, 0x8b}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, dz.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, dz.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, dz.periodsAbbreviated[0]...)
	} else {
		b = append(b, dz.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0xe0, 0xbd, 0x86, 0xe0, 0xbd, 0xb4, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xbc, 0xe0, 0xbd, 0x91, 0xe0, 0xbc, 0x8b, 0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, []byte{0x20, 0xe0, 0xbd, 0xa6, 0xe0, 0xbe, 0x90, 0xe0, 0xbd, 0xa2, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x98, 0xe0, 0xbc, 0x8b, 0x20}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, dz.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, dz.periodsAbbreviated[0]...)
	} else {
		b = append(b, dz.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'dz_BT'
func (dz *dz_BT) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0xe0, 0xbd, 0x86, 0xe0, 0xbd, 0xb4, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x9a, 0xe0, 0xbd, 0xbc, 0xe0, 0xbd, 0x91, 0xe0, 0xbc, 0x8b, 0x20}...)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, []byte{0x20, 0xe0, 0xbd, 0xa6, 0xe0, 0xbe, 0x90, 0xe0, 0xbd, 0xa2, 0xe0, 0xbc, 0x8b, 0xe0, 0xbd, 0x98, 0xe0, 0xbc, 0x8b, 0x20}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, dz.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, dz.periodsAbbreviated[0]...)
	} else {
		b = append(b, dz.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := dz.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
