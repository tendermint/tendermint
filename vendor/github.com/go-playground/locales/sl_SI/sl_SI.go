package sl_SI

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type sl_SI struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
	percentSuffix          string
	perMille               string
	timeSeparator          string
	inifinity              string
	currencies             []string // idx = enum of currency code
	currencyPositiveSuffix string
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

// New returns a new instance of translator for the 'sl_SI' locale
func New() locales.Translator {
	return &sl_SI{
		locale:                 "sl_SI",
		pluralsCardinal:        []locales.PluralRule{2, 3, 4, 6},
		pluralsOrdinal:         []locales.PluralRule{6},
		pluralsRange:           []locales.PluralRule{3, 4, 6},
		decimal:                ",",
		group:                  ".",
		minus:                  "−",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: " )",
		monthsAbbreviated:      []string{"", "jan.", "feb.", "mar.", "apr.", "maj", "jun.", "jul.", "avg.", "sep.", "okt.", "nov.", "dec."},
		monthsNarrow:           []string{"", "j", "f", "m", "a", "m", "j", "j", "a", "s", "o", "n", "d"},
		monthsWide:             []string{"", "januar", "februar", "marec", "april", "maj", "junij", "julij", "avgust", "september", "oktober", "november", "december"},
		daysAbbreviated:        []string{"ned.", "pon.", "tor.", "sre.", "čet.", "pet.", "sob."},
		daysNarrow:             []string{"n", "p", "t", "s", "č", "p", "s"},
		daysShort:              []string{"ned.", "pon.", "tor.", "sre.", "čet.", "pet.", "sob."},
		daysWide:               []string{"nedelja", "ponedeljek", "torek", "sreda", "četrtek", "petek", "sobota"},
		periodsAbbreviated:     []string{"dop.", "pop."},
		periodsNarrow:          []string{"d", "p"},
		periodsWide:            []string{"dop.", "pop."},
		erasAbbreviated:        []string{"pr. Kr.", "po Kr."},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"pred Kristusom", "po Kristusu"},
		timezones:              map[string]string{"CAT": "Centralnoafriški čas", "SRT": "Surinamski čas", "ACWST": "Avstralski centralni zahodni standardni čas", "TMST": "Turkmenistanski poletni čas", "ARST": "Argentinski poletni čas", "HEEG": "Vzhodnogrenlandski poletni čas", "GMT": "Greenwiški srednji čas", "CHAST": "Čatamski standardni čas", "HNPM": "Standardni čas: Saint Pierre in Miquelon", "MDT": "MDT", "IST": "Indijski standardni čas", "AEST": "Avstralski vzhodni standardni čas", "HAT": "Novofundlandski poletni čas", "COST": "Kolumbijski poletni čas", "LHDT": "Poletni čas otoka Lord Howe", "WARST": "Argentinski zahodni poletni čas", "CLT": "Čilski standardni čas", "ACST": "Avstralski centralni standardni čas", "CDT": "Centralni poletni čas", "HENOMX": "mehiški severozahodni poletni čas", "WAT": "Zahodnoafriški standardni čas", "HKT": "Hongkonški standardni čas", "HEPMX": "mehiški pacifiški poletni čas", "CST": "Centralni standardni čas", "AWST": "Avstralski zahodni standardni čas", "MEZ": "Srednjeevropski standardni čas", "TMT": "Turkmenistanski standardni čas", "SAST": "Južnoafriški čas", "GYT": "Gvajanski čas", "ACDT": "Avstralski centralni poletni čas", "∅∅∅": "Amazonski poletni čas", "WEZ": "Zahodnoevropski standardni čas", "OEZ": "Vzhodnoevropski standardni čas", "HNNOMX": "mehiški severozahodni standardni čas", "HNT": "Novofundlandski standardni čas", "ChST": "Čamorski standardni čas", "HNPMX": "mehiški pacifiški standardni čas", "UYT": "Urugvajski standardni čas", "LHST": "Standardni čas otoka Lord Howe", "WART": "Argentinski zahodni standardni čas", "AST": "Atlantski standardni čas", "WIB": "Indonezijski zahodni čas", "HEPM": "Poletni čas: Saint Pierre in Miquelon", "BT": "Butanski čas", "OESZ": "Vzhodnoevropski poletni čas", "ART": "Argentinski standardni čas", "ECT": "Ekvadorski čas", "HNCU": "Kubanski standardni čas", "AWDT": "Avstralski zahodni poletni čas", "MESZ": "Srednjeevropski poletni čas", "EST": "Vzhodni standardni čas", "ACWDT": "Avstralski centralni zahodni poletni čas", "MYT": "Malezijski čas", "HADT": "Havajski aleutski poletni čas", "WITA": "Indonezijski osrednji čas", "JDT": "Japonski poletni čas", "HNEG": "Vzhodnogrenlandski standardni čas", "WESZ": "Zahodnoevropski poletni čas", "UYST": "Urugvajski poletni čas", "EAT": "Vzhodnoafriški čas", "COT": "Kolumbijski standardni čas", "WAST": "Zahodnoafriški poletni čas", "CLST": "Čilski poletni čas", "AKDT": "Aljaški poletni čas", "PDT": "Pacifiški poletni čas", "CHADT": "Čatamski poletni čas", "HECU": "Kubanski poletni čas", "ADT": "Atlantski poletni čas", "AEDT": "Avstralski vzhodni poletni čas", "EDT": "Vzhodni poletni čas", "SGT": "Singapurski standardni čas", "NZDT": "Novozelandski poletni čas", "JST": "Japonski standardni čas", "GFT": "Čas: Francoska Gvajana", "AKST": "Aljaški standardni čas", "MST": "MST", "WIT": "Indonezijski vzhodni čas", "HAST": "Havajski aleutski standardni čas", "VET": "Venezuelski čas", "HNOG": "Zahodnogrenlandski standardni čas", "PST": "Pacifiški standardni čas", "BOT": "Bolivijski čas", "NZST": "Novozelandski standardni čas", "HEOG": "Zahodnogrenlandski poletni čas", "HKST": "Hongkonški poletni čas"},
	}
}

// Locale returns the current translators string locale
func (sl *sl_SI) Locale() string {
	return sl.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'sl_SI'
func (sl *sl_SI) PluralsCardinal() []locales.PluralRule {
	return sl.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'sl_SI'
func (sl *sl_SI) PluralsOrdinal() []locales.PluralRule {
	return sl.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'sl_SI'
func (sl *sl_SI) PluralsRange() []locales.PluralRule {
	return sl.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'sl_SI'
func (sl *sl_SI) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)
	iMod100 := i % 100

	if v == 0 && iMod100 == 1 {
		return locales.PluralRuleOne
	} else if v == 0 && iMod100 == 2 {
		return locales.PluralRuleTwo
	} else if (v == 0 && iMod100 >= 3 && iMod100 <= 4) || (v != 0) {
		return locales.PluralRuleFew
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'sl_SI'
func (sl *sl_SI) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'sl_SI'
func (sl *sl_SI) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := sl.CardinalPluralRule(num1, v1)
	end := sl.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOne {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleTwo {
		return locales.PluralRuleTwo
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleFew {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	} else if start == locales.PluralRuleTwo && end == locales.PluralRuleOne {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleTwo && end == locales.PluralRuleTwo {
		return locales.PluralRuleTwo
	} else if start == locales.PluralRuleTwo && end == locales.PluralRuleFew {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleTwo && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	} else if start == locales.PluralRuleFew && end == locales.PluralRuleOne {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleFew && end == locales.PluralRuleTwo {
		return locales.PluralRuleTwo
	} else if start == locales.PluralRuleFew && end == locales.PluralRuleFew {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleFew && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	} else if start == locales.PluralRuleOther && end == locales.PluralRuleOne {
		return locales.PluralRuleFew
	} else if start == locales.PluralRuleOther && end == locales.PluralRuleTwo {
		return locales.PluralRuleTwo
	} else if start == locales.PluralRuleOther && end == locales.PluralRuleFew {
		return locales.PluralRuleFew
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (sl *sl_SI) MonthAbbreviated(month time.Month) string {
	return sl.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (sl *sl_SI) MonthsAbbreviated() []string {
	return sl.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (sl *sl_SI) MonthNarrow(month time.Month) string {
	return sl.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (sl *sl_SI) MonthsNarrow() []string {
	return sl.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (sl *sl_SI) MonthWide(month time.Month) string {
	return sl.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (sl *sl_SI) MonthsWide() []string {
	return sl.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (sl *sl_SI) WeekdayAbbreviated(weekday time.Weekday) string {
	return sl.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (sl *sl_SI) WeekdaysAbbreviated() []string {
	return sl.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (sl *sl_SI) WeekdayNarrow(weekday time.Weekday) string {
	return sl.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (sl *sl_SI) WeekdaysNarrow() []string {
	return sl.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (sl *sl_SI) WeekdayShort(weekday time.Weekday) string {
	return sl.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (sl *sl_SI) WeekdaysShort() []string {
	return sl.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (sl *sl_SI) WeekdayWide(weekday time.Weekday) string {
	return sl.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (sl *sl_SI) WeekdaysWide() []string {
	return sl.daysWide
}

// Decimal returns the decimal point of number
func (sl *sl_SI) Decimal() string {
	return sl.decimal
}

// Group returns the group of number
func (sl *sl_SI) Group() string {
	return sl.group
}

// Group returns the minus sign of number
func (sl *sl_SI) Minus() string {
	return sl.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'sl_SI' and handles both Whole and Real numbers based on 'v'
func (sl *sl_SI) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sl.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, sl.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(sl.minus) - 1; j >= 0; j-- {
			b = append(b, sl.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'sl_SI' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (sl *sl_SI) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 7
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sl.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(sl.minus) - 1; j >= 0; j-- {
			b = append(b, sl.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, sl.percentSuffix...)

	b = append(b, sl.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'sl_SI'
func (sl *sl_SI) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := sl.currencies[currency]
	l := len(s) + len(symbol) + 6 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sl.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, sl.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(sl.minus) - 1; j >= 0; j-- {
			b = append(b, sl.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, sl.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, sl.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'sl_SI'
// in accounting notation.
func (sl *sl_SI) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := sl.currencies[currency]
	l := len(s) + len(symbol) + 8 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sl.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, sl.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, sl.currencyNegativePrefix[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, sl.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, sl.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, sl.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2e, 0x20}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, sl.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, sl.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, sl.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, sl.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'sl_SI'
func (sl *sl_SI) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, sl.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := sl.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
