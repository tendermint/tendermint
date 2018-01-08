package gu_IN

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type gu_IN struct {
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

// New returns a new instance of translator for the 'gu_IN' locale
func New() locales.Translator {
	return &gu_IN{
		locale:             "gu_IN",
		pluralsCardinal:    []locales.PluralRule{2, 6},
		pluralsOrdinal:     []locales.PluralRule{2, 3, 4, 5, 6},
		pluralsRange:       []locales.PluralRule{2, 6},
		decimal:            ".",
		group:              ",",
		minus:              "-",
		percent:            "%",
		perMille:           "‰",
		timeSeparator:      ":",
		inifinity:          "∞",
		currencies:         []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		monthsAbbreviated:  []string{"", "જાન્યુ", "ફેબ્રુ", "માર્ચ", "એપ્રિલ", "મે", "જૂન", "જુલાઈ", "ઑગસ્ટ", "સપ્ટે", "ઑક્ટો", "નવે", "ડિસે"},
		monthsNarrow:       []string{"", "જા", "ફે", "મા", "એ", "મે", "જૂ", "જુ", "ઑ", "સ", "ઑ", "ન", "ડિ"},
		monthsWide:         []string{"", "જાન્યુઆરી", "ફેબ્રુઆરી", "માર્ચ", "એપ્રિલ", "મે", "જૂન", "જુલાઈ", "ઑગસ્ટ", "સપ્ટેમ્બર", "ઑક્ટોબર", "નવેમ્બર", "ડિસેમ્બર"},
		daysAbbreviated:    []string{"રવિ", "સોમ", "મંગળ", "બુધ", "ગુરુ", "શુક્ર", "શનિ"},
		daysNarrow:         []string{"ર", "સો", "મં", "બુ", "ગુ", "શુ", "શ"},
		daysShort:          []string{"ર", "સો", "મં", "બુ", "ગુ", "શુ", "શ"},
		daysWide:           []string{"રવિવાર", "સોમવાર", "મંગળવાર", "બુધવાર", "ગુરુવાર", "શુક્રવાર", "શનિવાર"},
		periodsAbbreviated: []string{"AM", "PM"},
		periodsNarrow:      []string{"AM", "PM"},
		periodsWide:        []string{"AM", "PM"},
		erasAbbreviated:    []string{"ઈ.સ.પૂર્વે", "ઈ.સ."},
		erasNarrow:         []string{"ઇ સ પુ", "ઇસ"},
		erasWide:           []string{"ઈસવીસન પૂર્વે", "ઇસવીસન"},
		timezones:          map[string]string{"ECT": "એક્વાડોર સમય", "PDT": "ઉત્તર અમેરિકન પેસિફિક દિવસ સમય", "AWDT": "ઓસ્ટ્રેલિયન પશ્ચિમી દિવસ સમય", "MEZ": "મધ્ય યુરોપિયન માનક સમય", "HEOG": "પશ્ચિમ ગ્રીનલેન્ડ ગ્રીષ્મ સમય", "ADT": "અટલાન્ટિક દિવસ સમય", "AKDT": "અલાસ્કા દિવસ સમય", "VET": "વેનેઝુએલા સમય", "LHDT": "લોર્ડ હોવ દિવસ સમય", "WAT": "પશ્ચિમ આફ્રિકા માનક સમય", "COT": "કોલંબિયા માનક સમય", "HNNOMX": "ઉત્તરપશ્ચિમ મેક્સિકો માનક સમય", "HENOMX": "ઉત્તરપશ્ચિમ મેક્સિકો દિવસ સમય", "AST": "અટલાન્ટિક પ્રમાણભૂત સમય", "WIT": "પૂર્વીય ઇન્ડોનેશિયા સમય", "HEEG": "પૂર્વ ગ્રીનલેન્ડ ગ્રીષ્મ સમય", "WIB": "પશ્ચિમી ઇન્ડોનેશિયા સમય", "PST": "ઉત્તર અમેરિકન પેસિફિક પ્રમાણભૂત સમય", "MESZ": "મધ્ય યુરોપિયન ગ્રીષ્મ સમય", "HNOG": "પશ્ચિમ ગ્રીનલેન્ડ માનક સમય", "HNEG": "પૂર્વ ગ્રીનલેન્ડ માનક સમય", "ACDT": "ઓસ્ટ્રેલિયન મધ્ય દિવસ સમય", "WARST": "પશ્ચિમી અર્જેન્ટીના ગ્રીષ્મ સમય", "OESZ": "પૂર્વી યુરોપીયન ગ્રીષ્મ સમય", "LHST": "લોર્ડ હોવ પ્રમાણભૂત સમય", "IST": "ભારતીય માનક સમય", "COST": "કોલંબિયા ગ્રીષ્મ સમય", "HKST": "હોંગ કોંગ ગ્રીષ્મ સમય", "UYT": "ઉરૂગ્વે માનક સમય", "WART": "પશ્ચિમી અર્જેન્ટીના માનક સમય", "SAST": "દક્ષિણ આફ્રિકા માનક સમય", "CLT": "ચિલી માનક સમય", "MST": "મકાઉ પ્રમાણભૂત સમય", "EAT": "પૂર્વ આફ્રિકા સમય", "ACST": "ઓસ્ટ્રેલિયન મધ્ય પ્રમાણભૂત સમય", "EDT": "ઉત્તર અમેરિકન પૂર્વી દિવસ સમય", "CAT": "મધ્ય આફ્રિકા સમય", "ACWST": "ઓસ્ટ્રેલિયન મધ્ય પશ્ચિમી પ્રમાણભૂત સમય", "NZDT": "ન્યુઝીલેન્ડ દિવસ સમય", "TMST": "તુર્કમેનિસ્તાન ગ્રીષ્મ સમય", "JST": "જાપાન માનક સમય", "ARST": "આર્જેન્ટીના ગ્રીષ્મ સમય", "CLST": "ચિલી ગ્રીષ્મ સમય", "CHADT": "ચેતહામ દિવસ સમય", "NZST": "ન્યુઝીલેન્ડ માનક સમય", "TMT": "તુર્કમેનિસ્તાન માનક સમય", "AEDT": "ઓસ્ટ્રેલિયન પૂર્વીય દિવસ સમય", "AKST": "અલાસ્કા પ્રમાણભૂત સમય", "HNCU": "ક્યૂબા માનક સમય", "CHAST": "ચેતહામ માનક સમય", "BT": "ભૂટાન સમય", "HADT": "હવાઇ-એલ્યુશિઅન દિવસ સમય", "WAST": "પશ્ચિમ આફ્રિકા ગ્રીષ્મ સમય", "WESZ": "પશ્ચિમી યુરોપિયન ગ્રીષ્મ સમય", "AWST": "ઓસ્ટ્રેલિયન પશ્ચિમી પ્રમાણભૂત સમય", "CDT": "ઉત્તર અમેરિકન મધ્ય દિવસ સમય", "SGT": "સિંગાપુર માનક સમય", "HNPM": "સેંટ પીએરે એન્ડ મિકીલોન માનક સમય", "HECU": "ક્યૂબા દિવસ સમય", "AEST": "ઓસ્ટ્રેલિયન પૂર્વીય પ્રમાણભૂત સમય", "HKT": "હોંગ કોંગ માનક સમય", "SRT": "સૂરીનામ સમય", "CST": "ઉત્તર અમેરિકન કેન્દ્રિય પ્રમાણભૂત સમય", "OEZ": "પૂર્વી યુરોપિયન માનક સમય", "HNT": "ન્યૂફાઉન્ડલેન્ડ પ્રમાણભૂત સમય", "EST": "ઉત્તર અમેરિકન પૂર્વી પ્રમાણભૂત સમય", "HAST": "હવાઇ-એલ્યુશિઅન માનક સમય", "GFT": "ફ્રેન્ચ ગયાના સમય", "GYT": "ગયાના સમય", "HEPM": "સેંટ પીએરે એન્ડ મિકીલોન દિવસ સમય", "BOT": "બોલિવિયા સમય", "MDT": "મકાઉ ગ્રીષ્મ સમય", "MYT": "મલેશિયા સમય", "JDT": "જાપાન દિવસ સમય", "HAT": "ન્યૂફાઉન્ડલેન્ડ દિવસ સમય", "∅∅∅": "એમેઝોન ગ્રીષ્મ સમય", "HNPMX": "મેક્સીકન પેસિફિક માનક સમય", "HEPMX": "મેક્સીકન પેસિફિક દિવસ સમય", "GMT": "ગ્રીનવિચ મધ્યમ સમય", "ChST": "કેમોરો માનક સમય", "UYST": "ઉરૂગ્વે ગ્રીષ્મ સમય", "ACWDT": "ઓસ્ટ્રેલિયન મધ્ય પશ્ચિમી દિવસ સમય", "ART": "અર્જેન્ટીના માનક સમય", "WEZ": "પશ્ચિમી યુરોપિયન માનક સમય", "WITA": "મધ્ય ઇન્ડોનેશિયા સમય"},
	}
}

// Locale returns the current translators string locale
func (gu *gu_IN) Locale() string {
	return gu.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'gu_IN'
func (gu *gu_IN) PluralsCardinal() []locales.PluralRule {
	return gu.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'gu_IN'
func (gu *gu_IN) PluralsOrdinal() []locales.PluralRule {
	return gu.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'gu_IN'
func (gu *gu_IN) PluralsRange() []locales.PluralRule {
	return gu.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'gu_IN'
func (gu *gu_IN) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if (i == 0) || (n == 1) {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'gu_IN'
func (gu *gu_IN) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
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

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'gu_IN'
func (gu *gu_IN) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := gu.CardinalPluralRule(num1, v1)
	end := gu.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOne {
		return locales.PluralRuleOne
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (gu *gu_IN) MonthAbbreviated(month time.Month) string {
	return gu.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (gu *gu_IN) MonthsAbbreviated() []string {
	return gu.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (gu *gu_IN) MonthNarrow(month time.Month) string {
	return gu.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (gu *gu_IN) MonthsNarrow() []string {
	return gu.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (gu *gu_IN) MonthWide(month time.Month) string {
	return gu.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (gu *gu_IN) MonthsWide() []string {
	return gu.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (gu *gu_IN) WeekdayAbbreviated(weekday time.Weekday) string {
	return gu.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (gu *gu_IN) WeekdaysAbbreviated() []string {
	return gu.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (gu *gu_IN) WeekdayNarrow(weekday time.Weekday) string {
	return gu.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (gu *gu_IN) WeekdaysNarrow() []string {
	return gu.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (gu *gu_IN) WeekdayShort(weekday time.Weekday) string {
	return gu.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (gu *gu_IN) WeekdaysShort() []string {
	return gu.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (gu *gu_IN) WeekdayWide(weekday time.Weekday) string {
	return gu.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (gu *gu_IN) WeekdaysWide() []string {
	return gu.daysWide
}

// Decimal returns the decimal point of number
func (gu *gu_IN) Decimal() string {
	return gu.decimal
}

// Group returns the group of number
func (gu *gu_IN) Group() string {
	return gu.group
}

// Group returns the minus sign of number
func (gu *gu_IN) Minus() string {
	return gu.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'gu_IN' and handles both Whole and Real numbers based on 'v'
func (gu *gu_IN) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	inSecondary := false
	groupThreshold := 3

	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, gu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {

			if count == groupThreshold {
				b = append(b, gu.group[0])
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
		b = append(b, gu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'gu_IN' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (gu *gu_IN) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, gu.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, gu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, gu.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'gu_IN'
func (gu *gu_IN) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := gu.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, gu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, gu.group[0])
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
		b = append(b, gu.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, gu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'gu_IN'
// in accounting notation.
func (gu *gu_IN) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := gu.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, gu.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, gu.group[0])
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

		b = append(b, gu.minus[0])

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
			b = append(b, gu.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, gu.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, gu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, gu.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, gu.monthsWide[t.Month()]...)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	if h < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, gu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, gu.periodsAbbreviated[0]...)
	} else {
		b = append(b, gu.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	if h < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, gu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, gu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, gu.periodsAbbreviated[0]...)
	} else {
		b = append(b, gu.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	if h < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, gu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, gu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, gu.periodsAbbreviated[0]...)
	} else {
		b = append(b, gu.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'gu_IN'
func (gu *gu_IN) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	if h < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, gu.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, gu.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, gu.periodsAbbreviated[0]...)
	} else {
		b = append(b, gu.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := gu.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
