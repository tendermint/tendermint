package fi_FI

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type fi_FI struct {
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

// New returns a new instance of translator for the 'fi_FI' locale
func New() locales.Translator {
	return &fi_FI{
		locale:                 "fi_FI",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{6},
		pluralsRange:           []locales.PluralRule{6},
		decimal:                ",",
		group:                  " ",
		minus:                  "−",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ".",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "tammik.", "helmik.", "maalisk.", "huhtik.", "toukok.", "kesäk.", "heinäk.", "elok.", "syysk.", "lokak.", "marrask.", "jouluk."},
		monthsNarrow:           []string{"", "T", "H", "M", "H", "T", "K", "H", "E", "S", "L", "M", "J"},
		monthsWide:             []string{"", "tammikuuta", "helmikuuta", "maaliskuuta", "huhtikuuta", "toukokuuta", "kesäkuuta", "heinäkuuta", "elokuuta", "syyskuuta", "lokakuuta", "marraskuuta", "joulukuuta"},
		daysAbbreviated:        []string{"su", "ma", "ti", "ke", "to", "pe", "la"},
		daysNarrow:             []string{"S", "M", "T", "K", "T", "P", "L"},
		daysShort:              []string{"su", "ma", "ti", "ke", "to", "pe", "la"},
		daysWide:               []string{"sunnuntaina", "maanantaina", "tiistaina", "keskiviikkona", "torstaina", "perjantaina", "lauantaina"},
		periodsAbbreviated:     []string{"ap.", "ip."},
		periodsNarrow:          []string{"ap.", "ip."},
		periodsWide:            []string{"ap.", "ip."},
		erasAbbreviated:        []string{"eKr.", "jKr."},
		erasNarrow:             []string{"eKr", "jKr"},
		erasWide:               []string{"ennen Kristuksen syntymää", "jälkeen Kristuksen syntymän"},
		timezones:              map[string]string{"WAST": "Länsi-Afrikan kesäaika", "HECU": "Kuuban kesäaika", "AWDT": "Länsi-Australian kesäaika", "ACWDT": "Läntisen Keski-Australian kesäaika", "UYST": "Uruguayn kesäaika", "JST": "Japanin normaaliaika", "HEOG": "Länsi-Grönlannin kesäaika", "SAST": "Etelä-Afrikan aika", "AEST": "Itä-Australian normaaliaika", "HAT": "Newfoundlandin kesäaika", "HNPM": "Saint-Pierren ja Miquelonin normaaliaika", "MESZ": "Keski-Euroopan kesäaika", "ADT": "Kanadan Atlantin kesäaika", "HNEG": "Itä-Grönlannin normaaliaika", "CLT": "Chilen normaaliaika", "CAT": "Keski-Afrikan aika", "CHADT": "Chathamin kesäaika", "HKST": "Hongkongin kesäaika", "HEPMX": "Meksikon Tyynenmeren kesäaika", "MDT": "Macaon kesäaika", "HAST": "Havaijin-Aleuttien normaaliaika", "HNNOMX": "Luoteis-Meksikon normaaliaika", "EAT": "Itä-Afrikan aika", "HNT": "Newfoundlandin normaaliaika", "CHAST": "Chathamin normaaliaika", "TMST": "Turkmenistanin kesäaika", "PST": "Yhdysvaltain Tyynenmeren normaaliaika", "OESZ": "Itä-Euroopan kesäaika", "WAT": "Länsi-Afrikan normaaliaika", "COT": "Kolumbian normaaliaika", "ACST": "Keski-Australian normaaliaika", "ChST": "Tšamorron aika", "MEZ": "Keski-Euroopan normaaliaika", "NZST": "Uuden-Seelannin normaaliaika", "AST": "Kanadan Atlantin normaaliaika", "GYT": "Guyanan aika", "GMT": "Greenwichin normaaliaika", "WIB": "Länsi-Indonesian aika", "BOT": "Bolivian aika", "HEEG": "Itä-Grönlannin kesäaika", "HEPM": "Saint-Pierren ja Miquelonin kesäaika", "ACWST": "Läntisen Keski-Australian normaaliaika", "WIT": "Itä-Indonesian aika", "GFT": "Ranskan Guayanan aika", "WEZ": "Länsi-Euroopan normaaliaika", "AWST": "Länsi-Australian normaaliaika", "NZDT": "Uuden-Seelannin kesäaika", "LHDT": "Lord Howen kesäaika", "WARST": "Länsi-Argentiinan kesäaika", "AEDT": "Itä-Australian kesäaika", "ACDT": "Keski-Australian kesäaika", "HNCU": "Kuuban normaaliaika", "CDT": "Yhdysvaltain keskinen kesäaika", "MYT": "Malesian aika", "UYT": "Uruguayn normaaliaika", "OEZ": "Itä-Euroopan normaaliaika", "WART": "Länsi-Argentiinan normaaliaika", "AKDT": "Alaskan kesäaika", "HENOMX": "Luoteis-Meksikon kesäaika", "LHST": "Lord Howen normaaliaika", "IST": "Intian aika", "ARST": "Argentiinan kesäaika", "WESZ": "Länsi-Euroopan kesäaika", "∅∅∅": "Brasilian kesäaika", "HADT": "Havaijin-Aleuttien kesäaika", "ART": "Argentiinan normaaliaika", "COST": "Kolumbian kesäaika", "EDT": "Yhdysvaltain itäinen kesäaika", "AKST": "Alaskan normaaliaika", "CLST": "Chilen kesäaika", "BT": "Bhutanin aika", "CST": "Yhdysvaltain keskinen normaaliaika", "MST": "Macaon normaaliaika", "TMT": "Turkmenistanin normaaliaika", "WITA": "Keski-Indonesian aika", "JDT": "Japanin kesäaika", "HNOG": "Länsi-Grönlannin normaaliaika", "EST": "Yhdysvaltain itäinen normaaliaika", "ECT": "Ecuadorin aika", "SGT": "Singaporen aika", "HNPMX": "Meksikon Tyynenmeren normaaliaika", "PDT": "Yhdysvaltain Tyynenmeren kesäaika", "SRT": "Surinamen aika", "VET": "Venezuelan aika", "HKT": "Hongkongin normaaliaika"},
	}
}

// Locale returns the current translators string locale
func (fi *fi_FI) Locale() string {
	return fi.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'fi_FI'
func (fi *fi_FI) PluralsCardinal() []locales.PluralRule {
	return fi.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'fi_FI'
func (fi *fi_FI) PluralsOrdinal() []locales.PluralRule {
	return fi.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'fi_FI'
func (fi *fi_FI) PluralsRange() []locales.PluralRule {
	return fi.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'fi_FI'
func (fi *fi_FI) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 1 && v == 0 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'fi_FI'
func (fi *fi_FI) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'fi_FI'
func (fi *fi_FI) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (fi *fi_FI) MonthAbbreviated(month time.Month) string {
	return fi.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (fi *fi_FI) MonthsAbbreviated() []string {
	return fi.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (fi *fi_FI) MonthNarrow(month time.Month) string {
	return fi.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (fi *fi_FI) MonthsNarrow() []string {
	return fi.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (fi *fi_FI) MonthWide(month time.Month) string {
	return fi.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (fi *fi_FI) MonthsWide() []string {
	return fi.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (fi *fi_FI) WeekdayAbbreviated(weekday time.Weekday) string {
	return fi.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (fi *fi_FI) WeekdaysAbbreviated() []string {
	return fi.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (fi *fi_FI) WeekdayNarrow(weekday time.Weekday) string {
	return fi.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (fi *fi_FI) WeekdaysNarrow() []string {
	return fi.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (fi *fi_FI) WeekdayShort(weekday time.Weekday) string {
	return fi.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (fi *fi_FI) WeekdaysShort() []string {
	return fi.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (fi *fi_FI) WeekdayWide(weekday time.Weekday) string {
	return fi.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (fi *fi_FI) WeekdaysWide() []string {
	return fi.daysWide
}

// Decimal returns the decimal point of number
func (fi *fi_FI) Decimal() string {
	return fi.decimal
}

// Group returns the group of number
func (fi *fi_FI) Group() string {
	return fi.group
}

// Group returns the minus sign of number
func (fi *fi_FI) Minus() string {
	return fi.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'fi_FI' and handles both Whole and Real numbers based on 'v'
func (fi *fi_FI) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fi.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(fi.group) - 1; j >= 0; j-- {
					b = append(b, fi.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(fi.minus) - 1; j >= 0; j-- {
			b = append(b, fi.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'fi_FI' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (fi *fi_FI) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 7
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fi.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(fi.minus) - 1; j >= 0; j-- {
			b = append(b, fi.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, fi.percentSuffix...)

	b = append(b, fi.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'fi_FI'
func (fi *fi_FI) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := fi.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fi.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(fi.group) - 1; j >= 0; j-- {
					b = append(b, fi.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(fi.minus) - 1; j >= 0; j-- {
			b = append(b, fi.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, fi.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, fi.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'fi_FI'
// in accounting notation.
func (fi *fi_FI) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := fi.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fi.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(fi.group) - 1; j >= 0; j-- {
					b = append(b, fi.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(fi.minus) - 1; j >= 0; j-- {
			b = append(b, fi.minus[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, fi.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, fi.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, fi.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, fi.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, []byte{0x63, 0x63, 0x63, 0x63, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, fi.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'fi_FI'
func (fi *fi_FI) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := fi.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
