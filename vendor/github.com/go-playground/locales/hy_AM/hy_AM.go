package hy_AM

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type hy_AM struct {
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

// New returns a new instance of translator for the 'hy_AM' locale
func New() locales.Translator {
	return &hy_AM{
		locale:                 "hy_AM",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{2, 6},
		pluralsRange:           []locales.PluralRule{2, 6},
		decimal:                ",",
		group:                  " ",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositivePrefix: " ",
		currencyNegativePrefix: " ",
		monthsAbbreviated:      []string{"", "հնվ", "փտվ", "մրտ", "ապր", "մյս", "հնս", "հլս", "օգս", "սեպ", "հոկ", "նոյ", "դեկ"},
		monthsNarrow:           []string{"", "Հ", "Փ", "Մ", "Ա", "Մ", "Հ", "Հ", "Օ", "Ս", "Հ", "Ն", "Դ"},
		monthsWide:             []string{"", "հունվարի", "փետրվարի", "մարտի", "ապրիլի", "մայիսի", "հունիսի", "հուլիսի", "օգոստոսի", "սեպտեմբերի", "հոկտեմբերի", "նոյեմբերի", "դեկտեմբերի"},
		daysAbbreviated:        []string{"կիր", "երկ", "երք", "չրք", "հնգ", "ուր", "շբթ"},
		daysNarrow:             []string{"Կ", "Ե", "Ե", "Չ", "Հ", "Ո", "Շ"},
		daysShort:              []string{"կր", "եկ", "եք", "չք", "հգ", "ու", "շբ"},
		daysWide:               []string{"կիրակի", "երկուշաբթի", "երեքշաբթի", "չորեքշաբթի", "հինգշաբթի", "ուրբաթ", "շաբաթ"},
		periodsAbbreviated:     []string{"ԿԱ", "ԿՀ"},
		periodsNarrow:          []string{"ա", "հ"},
		periodsWide:            []string{"AM", "PM"},
		erasAbbreviated:        []string{"", ""},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"Քրիստոսից առաջ", "Քրիստոսից հետո"},
		timezones:              map[string]string{"VET": "Վենեսուելայի ժամանակ", "COT": "Կոլումբիայի ստանդարտ ժամանակ", "HKT": "Հոնկոնգի ստանդարտ ժամանակ", "WESZ": "Արևմտյան Եվրոպայի ամառային ժամանակ", "SRT": "Սուրինամի ժամանակ", "AEST": "Արևելյան Ավստրալիայի ստանդարտ ժամանակ", "EAT": "Արևելյան Աֆրիկայի ժամանակ", "AKDT": "Ալյասկայի ամառային ժամանակ", "CAT": "Կենտրոնական Աֆրիկայի ժամանակ", "SGT": "Սինգապուրի ժամանակ", "ACWST": "Կենտրոնական Ավստրալիայի արևմտյան ստանդարտ ժամանակ", "JST": "Ճապոնիայի ստանդարտ ժամանակ", "ADT": "Ատլանտյան ամառային ժամանակ", "SAST": "Հարավային Աֆրիկայի ժամանակ", "GFT": "Ֆրանսիական Գվիանայի ժամանակ", "HNPMX": "Մեքսիկայի խաղաղօվկիանոսյան ստանդարտ ժամանակ", "BOT": "Բոլիվիայի ժամանակ", "MST": "MST", "NZST": "Նոր Զելանդիայի ստանդարտ ժամանակ", "HENOMX": "Հյուսիսարևմտյան Մեքսիկայի ամառային ժամանակ", "ARST": "Արգենտինայի ամառային ժամանակ", "ACDT": "Կենտրոնական Ավստրալիայի ամառային ժամանակ", "WEZ": "Արևմտյան Եվրոպայի ստանդարտ ժամանակ", "CHADT": "Չաթեմ կղզու ամառային ժամանակ", "HECU": "Կուբայի ամառային ժամանակ", "MEZ": "Կենտրոնական Եվրոպայի ստանդարտ ժամանակ", "IST": "Հնդկաստանի ստանդարտ ժամանակ", "HNOG": "Արևմտյան Գրենլանդիայի ստանդարտ ժամանակ", "EDT": "Արևելյան Ամերիկայի ամառային ժամանակ", "MDT": "MDT", "WIT": "Արևելյան Ինդոնեզիայի ժամանակ", "ART": "Արգենտինայի ստնադարտ ժամանակ", "COST": "Կոլումբիայի ամառային ժամանակ", "PST": "Խաղաղօվկիանոսյան ստանդարտ ժամանակ", "HNCU": "Կուբայի ստանդարտ ժամանակ", "MESZ": "Կենտրոնական Եվրոպայի ամառային ժամանակ", "WART": "Արևմտյան Արգենտինայի ստնադարտ ժամանակ", "HKST": "Հոնկոնգի ամառային ժամանակ", "PDT": "Խաղաղօվկիանոսյան ամառային ժամանակ", "OESZ": "Արևելյան Եվրոպայի ամառային ժամանակ", "WAST": "Արևմտյան Աֆրիկայի ամառային ժամանակ", "GYT": "Գայանայի ժամանակ", "ACWDT": "Կենտրոնական Ավստրալիայի արևմտյան ամառային ժամանակ", "HNNOMX": "Հյուսիսարևմտյան Մեքսիկայի ստանդարտ ժամանակ", "LHST": "Լորդ Հաուի ստանդարտ ժամանակ", "LHDT": "Լորդ Հաուի ամառային ժամանակ", "CLST": "Չիլիի ամառային ժամանակ", "EST": "Արևելյան Ամերիկայի ստանդարտ ժամանակ", "HNPM": "Սեն Պիեռ և Միքելոնի ստանդարտ ժամանակ", "AWST": "Արևմտյան Ավստրալիայի ստանդարտ ժամանակ", "HADT": "Հավայան-ալեության ամառային ժամանակ", "NZDT": "Նոր Զելանդիայի ամառային ժամանակ", "HAT": "Նյուֆաունդլենդի ամառային ժամանակ", "WIB": "Արևմտյան Ինդոնեզիայի ժամանակ", "ChST": "Չամոռոյի ժամանակ", "MYT": "Մալայզիայի ժամանակ", "WITA": "Կենտրոնական Ինդոնեզիայի ժամանակ", "HEEG": "Արևելյան Գրենլանդիայի ամառային ժամանակ", "ACST": "Կենտրոնական Ավստրալիայի ստանդարտ ժամանակ", "JDT": "Ճապոնիայի ամառային ժամանակ", "AEDT": "Արևելյան Ավստրալիայի ամառային ժամանակ", "∅∅∅": "Պերուի ամառային ժամանակ", "HEPM": "Սեն Պիեռ և Միքելոնի ամառային ժամանակ", "CDT": "Կենտրոնական Ամերիկայի ամառային ժամանակ", "UYST": "Ուրուգվայի ամառային ժամանակ", "TMT": "Թուրքմենստանի ստանդարտ ժամանակ", "WARST": "Արևմտյան Արգենտինայի ամառային ժամանակ", "AKST": "Ալյասկայի ստանդարտ ժամանակ", "HEPMX": "Մեքսիկայի խաղաղօվկիանոսյան ամառային ժամանակ", "CHAST": "Չաթեմ կղզու ստանդարտ ժամանակ", "CST": "Կենտրոնական Ամերիկայի ստանդարտ ժամանակ", "TMST": "Թուրքմենստանի ամառային ժամանակ", "HEOG": "Արևմտյան Գրենլանդիայի ամառային ժամանակ", "AST": "Ատլանտյան ստանդարտ ժամանակ", "GMT": "Գրինվիչի ժամանակ", "AWDT": "Արևմտյան Ավստրալիայի ամառային ժամանակ", "ECT": "Էկվադորի ժամանակ", "BT": "Բութանի ժամանակ", "UYT": "Ուրուգվայի ստանդարտ ժամանակ", "OEZ": "Արևելյան Եվրոպայի ստանդարտ ժամանակ", "WAT": "Արևմտյան Աֆրիկայի ստանդարտ ժամանակ", "HNT": "Նյուֆաունդլենդի ստանդարտ ժամանակ", "CLT": "Չիլիի ստանդարտ ժամանակ", "HAST": "Հավայան-ալեության ստանդարտ ժամանակ", "HNEG": "Արևելյան Գրենլանդիայի ստանդարտ ժամանակ"},
	}
}

// Locale returns the current translators string locale
func (hy *hy_AM) Locale() string {
	return hy.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'hy_AM'
func (hy *hy_AM) PluralsCardinal() []locales.PluralRule {
	return hy.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'hy_AM'
func (hy *hy_AM) PluralsOrdinal() []locales.PluralRule {
	return hy.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'hy_AM'
func (hy *hy_AM) PluralsRange() []locales.PluralRule {
	return hy.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'hy_AM'
func (hy *hy_AM) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 0 || i == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'hy_AM'
func (hy *hy_AM) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'hy_AM'
func (hy *hy_AM) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := hy.CardinalPluralRule(num1, v1)
	end := hy.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOne {
		return locales.PluralRuleOne
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (hy *hy_AM) MonthAbbreviated(month time.Month) string {
	return hy.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (hy *hy_AM) MonthsAbbreviated() []string {
	return hy.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (hy *hy_AM) MonthNarrow(month time.Month) string {
	return hy.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (hy *hy_AM) MonthsNarrow() []string {
	return hy.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (hy *hy_AM) MonthWide(month time.Month) string {
	return hy.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (hy *hy_AM) MonthsWide() []string {
	return hy.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (hy *hy_AM) WeekdayAbbreviated(weekday time.Weekday) string {
	return hy.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (hy *hy_AM) WeekdaysAbbreviated() []string {
	return hy.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (hy *hy_AM) WeekdayNarrow(weekday time.Weekday) string {
	return hy.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (hy *hy_AM) WeekdaysNarrow() []string {
	return hy.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (hy *hy_AM) WeekdayShort(weekday time.Weekday) string {
	return hy.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (hy *hy_AM) WeekdaysShort() []string {
	return hy.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (hy *hy_AM) WeekdayWide(weekday time.Weekday) string {
	return hy.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (hy *hy_AM) WeekdaysWide() []string {
	return hy.daysWide
}

// Decimal returns the decimal point of number
func (hy *hy_AM) Decimal() string {
	return hy.decimal
}

// Group returns the group of number
func (hy *hy_AM) Group() string {
	return hy.group
}

// Group returns the minus sign of number
func (hy *hy_AM) Minus() string {
	return hy.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'hy_AM' and handles both Whole and Real numbers based on 'v'
func (hy *hy_AM) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, hy.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(hy.group) - 1; j >= 0; j-- {
					b = append(b, hy.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, hy.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'hy_AM' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (hy *hy_AM) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, hy.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, hy.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, hy.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'hy_AM'
func (hy *hy_AM) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := hy.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, hy.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(hy.group) - 1; j >= 0; j-- {
					b = append(b, hy.group[j])
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

	for j := len(hy.currencyPositivePrefix) - 1; j >= 0; j-- {
		b = append(b, hy.currencyPositivePrefix[j])
	}

	if num < 0 {
		b = append(b, hy.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, hy.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'hy_AM'
// in accounting notation.
func (hy *hy_AM) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := hy.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, hy.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(hy.group) - 1; j >= 0; j-- {
					b = append(b, hy.group[j])
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

		for j := len(hy.currencyNegativePrefix) - 1; j >= 0; j-- {
			b = append(b, hy.currencyNegativePrefix[j])
		}

		b = append(b, hy.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(hy.currencyPositivePrefix) - 1; j >= 0; j-- {
			b = append(b, hy.currencyPositivePrefix[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, hy.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2e}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, hy.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20, 0xd5, 0xa9, 0x2e}...)

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, hy.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20, 0xd5, 0xa9, 0x2e}...)

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20, 0xd5, 0xa9, 0x2e, 0x20}...)
	b = append(b, hy.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, hy.daysWide[t.Weekday()]...)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'hy_AM'
func (hy *hy_AM) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, hy.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := hy.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
