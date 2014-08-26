package domainstats

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dead10ck/goinvestigate"
)

// Takes a struct that consists of just bool fields
// and returns true if any of the fields are true
func any(structField interface{}) bool {
	rType := reflect.TypeOf(structField)
	rInterfaceVal := reflect.ValueOf(structField)
	rVal := rInterfaceVal.Convert(rType)
	for i := 0; i < rVal.NumField(); i++ {
		if rVal.Field(i).Bool() {
			return true
		}
	}
	return false
}

func (c *Config) DeriveHeader() (header []string) {
	appendField := func(field string, cond bool) {
		if cond {
			header = append(header, field)
		}
	}

	appendFields := func(structField interface{}) {
		rType := reflect.TypeOf(structField)
		rInterfaceVal := reflect.ValueOf(structField)
		rVal := rInterfaceVal.Convert(rType)
		for i := 0; i < rVal.NumField(); i++ {
			fieldVal := rVal.Type().Field(i)
			fieldName := fieldVal.Name
			appendField(fieldName, rVal.Field(i).Bool())
		}
	}

	// Add the static fields
	appendField("Status", c.Status)
	appendFields(c.Categories)
	appendFields(c.Security)
	appendFields(c.DomainRRHistory)

	// Add a single header field for each dynamic field
	if any(c.Cooccurrences) {
		appendField("Cooccurrences", true)
	}
	if any(c.Related) {
		appendField("RelatedDomains", true)
	}
	if any(c.TaggingDates) {
		appendField("TaggingDates", true)
	}

	return header
}

// Uses a goinvestigate response to derive the field values to go in a
// CSV row. Once all responses are processed with this function, the
// []string results can be concatenated to yield the final CSV row.
func (c *Config) ExtractCSVSubRow(goinvResp interface{}) (row []string, err error) {
	switch resp := goinvResp.(type) {
	case *goinvestigate.DomainCategorization:
		return c.extractDomainCatInfo(resp), nil
	case []goinvestigate.RelatedDomain:
		return c.extractRelatedDomainInfo(resp), nil
	case []goinvestigate.Cooccurrence:
		return c.extractCooccurrenceInfo(resp), nil
	case *goinvestigate.SecurityFeatures:
		return c.extractSecurityFeaturesInfo(resp), nil
	case []goinvestigate.DomainTag:
		return c.extractDomainTagInfo(resp), nil
	case *goinvestigate.DomainRRHistory:
		return c.extractDomainRRHistoryInfo(resp), nil
	default:
		return nil, errors.New("invalid type")
	}
}

func (c *Config) extractDomainCatInfo(resp *goinvestigate.DomainCategorization) []string {
	var row []string
	if c.Status {
		row = append(row, strconv.Itoa(resp.Status))
	}

	if c.Categories.SecurityCategories {
		row = append(row, strings.Join(resp.SecurityCategories, ", "))
	}

	if c.Categories.ContentCategories {
		row = append(row, strings.Join(resp.ContentCategories, ", "))
	}

	return row
}

// dynamic field. Should return a singleton list
func (c *Config) extractRelatedDomainInfo(resp []goinvestigate.RelatedDomain) []string {
	row := []string{}
	for _, rd := range resp {
		if c.Related.Domain {
			row = append(row, rd.Domain)
			if c.Related.Score {
				row[len(row)-1] += ":"
			}
		} else if c.Related.Score {
			row = append(row, "")
		}
		if c.Related.Score {
			row[len(row)-1] += strconv.Itoa(rd.Score)
		}
	}
	if len(row) == 0 {
		return []string{}
	}
	return []string{strings.Join(row, ", ")}
}

// dynamic field. Should return a singleton list
func (c *Config) extractCooccurrenceInfo(resp []goinvestigate.Cooccurrence) []string {
	row := []string{}
	for _, cooc := range resp {
		if c.Cooccurrences.Domain {
			row = append(row, cooc.Domain)
			if c.Cooccurrences.Score {
				row[len(row)-1] += ":"
			}
		} else if c.Cooccurrences.Score {
			row = append(row, "")
		}
		if c.Cooccurrences.Score {
			row[len(row)-1] += convertFloatToStr(cooc.Score)
		}
	}
	if len(row) == 0 {
		return []string{}
	}
	return []string{strings.Join(row, ", ")}
}

// partially dynamic. Geo* fields are single fields
func (c *Config) extractSecurityFeaturesInfo(resp *goinvestigate.SecurityFeatures) []string {
	row := []string{}
	row = appendIf(row, convertFloatToStr(resp.DGAScore), c.Security.DGAScore)
	row = appendIf(row, convertFloatToStr(resp.Perplexity), c.Security.Perplexity)
	row = appendIf(row, convertFloatToStr(resp.Entropy), c.Security.Entropy)
	row = appendIf(row, convertFloatToStr(resp.SecureRank2), c.Security.SecureRank2)
	row = appendIf(row, convertFloatToStr(resp.PageRank), c.Security.PageRank)
	row = appendIf(row, convertFloatToStr(resp.ASNScore), c.Security.ASNScore)
	row = appendIf(row, convertFloatToStr(resp.PrefixScore), c.Security.PrefixScore)
	row = appendIf(row, convertFloatToStr(resp.RIPScore), c.Security.RIPScore)
	row = appendIf(row, convertFloatToStr(resp.Popularity), c.Security.Popularity)
	row = appendIf(row, strconv.FormatBool(resp.Fastflux), c.Security.Fastflux)
	row = appendIf(row, GeoString(resp.Geodiversity), c.Security.Geodiversity)
	row = appendIf(row, GeoString(resp.GeodiversityNormalized), c.Security.GeodiversityNormalized)
	row = appendIf(row, GeoString(resp.TLDGeodiversity), c.Security.TLDGeodiversity)
	row = appendIf(row, convertFloatToStr(resp.Geoscore), c.Security.Geoscore)
	row = appendIf(row, convertFloatToStr(resp.KSTest), c.Security.KSTest)
	row = appendIf(row, resp.Attack, c.Security.Attack)
	row = appendIf(row, resp.ThreatType, c.Security.ThreatType)
	return row
}

// dynamic field. Should return a singleton list
func (c *Config) extractDomainTagInfo(resp []goinvestigate.DomainTag) []string {
	dtStrs := []string{}
	for _, dt := range resp {
		fieldStrs := []string{}
		fieldStrs = appendIf(fieldStrs, dt.Url, c.TaggingDates.Url)
		fieldStrs = appendIf(fieldStrs, dt.Category, c.TaggingDates.Category)
		fieldStrs = appendIf(fieldStrs, dt.Period.Begin, c.TaggingDates.Begin)
		fieldStrs = appendIf(fieldStrs, dt.Period.End, c.TaggingDates.End)
		if len(fieldStrs) != 0 {
			dtStrs = append(dtStrs, strings.Join(fieldStrs, ":"))
		}
	}
	return dtStrs
}

func (c *Config) extractDomainRRHistoryInfo(resp *goinvestigate.DomainRRHistory) []string {
	row := []string{}
	rrPeriodsStr := c.rrPeriodsToStr(resp.RRPeriods)
	row = appendIf(row, rrPeriodsStr, rrPeriodsStr != "")
	row = appendIf(row, strconv.Itoa(resp.RRFeatures.Age), c.DomainRRHistory.Age)
	row = appendIf(row, strconv.Itoa(resp.RRFeatures.TTLsMin), c.DomainRRHistory.TTLsMin)
	row = appendIf(row, strconv.Itoa(resp.RRFeatures.TTLsMax), c.DomainRRHistory.TTLsMax)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.TTLsMean), c.DomainRRHistory.TTLsMean)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.TTLsMedian), c.DomainRRHistory.TTLsMedian)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.TTLsStdDev), c.DomainRRHistory.TTLsStdDev)
	row = appendIf(row, strings.Join(resp.RRFeatures.CountryCodes, ", "), c.DomainRRHistory.CountryCodes)

	asnStrs := []string{}
	for _, asn := range resp.RRFeatures.ASNs {
		asnStrs = append(asnStrs, strconv.Itoa(asn))
	}

	row = appendIf(row, strings.Join(asnStrs, ", "), c.DomainRRHistory.ASNs)
	row = appendIf(row, strings.Join(resp.RRFeatures.Prefixes, ", "), c.DomainRRHistory.Prefixes)
	row = appendIf(row, strconv.Itoa(resp.RRFeatures.RIPSCount), c.DomainRRHistory.RIPSCount)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.RIPSDiversity), c.DomainRRHistory.RIPSDiversity)
	row = appendIf(row, locsToStr(resp.RRFeatures.Locations), c.DomainRRHistory.Locations)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.GeoDistanceSum), c.DomainRRHistory.GeoDistanceSum)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.GeoDistanceMean), c.DomainRRHistory.GeoDistanceMean)
	row = appendIf(row, strconv.FormatBool(resp.RRFeatures.NonRoutable), c.DomainRRHistory.NonRoutable)
	row = appendIf(row, strconv.FormatBool(resp.RRFeatures.MailExchanger), c.DomainRRHistory.MailExchanger)
	row = appendIf(row, strconv.FormatBool(resp.RRFeatures.CName), c.DomainRRHistory.CName)
	row = appendIf(row, strconv.FormatBool(resp.RRFeatures.FFCandidate), c.DomainRRHistory.FFCandidate)
	row = appendIf(row, convertFloatToStr(resp.RRFeatures.RIPSStability), c.DomainRRHistory.RIPSStability)
	return row
}

func locsToStr(locs []goinvestigate.Location) string {
	strs := []string{}
	for _, loc := range locs {
		locStrs := []string{}
		locStrs = append(locStrs, convertFloatToStr(loc.Lat))
		locStrs = append(locStrs, convertFloatToStr(loc.Lon))
		strs = append(strs, strings.Join(locStrs, ":"))
	}
	return strings.Join(strs, ", ")
}

func (c *Config) rrPeriodsToStr(periods []goinvestigate.ResourceRecordPeriod) string {
	periodStrs := []string{}
	for _, p := range periods {
		flStrs := []string{}
		flStrs = appendIf(flStrs, p.FirstSeen, c.DomainRRHistory.FirstSeen)
		flStrs = appendIf(flStrs, p.LastSeen, c.DomainRRHistory.LastSeen)
		for _, rr := range p.RRs {
			rrStrs := []string{}
			rrStrs = appendIf(rrStrs, rr.Name, c.DomainRRHistory.Name)
			rrStrs = appendIf(rrStrs, strconv.Itoa(rr.TTL), c.DomainRRHistory.TTL)
			rrStrs = appendIf(rrStrs, rr.Class, c.DomainRRHistory.Class)
			rrStrs = appendIf(rrStrs, rr.Type, c.DomainRRHistory.Type)
			rrStrs = appendIf(rrStrs, rr.RR, c.DomainRRHistory.RR)

			flrr := append(flStrs, rrStrs...)
			if len(flrr) != 0 {
				periodStrs = append(periodStrs, strings.Join(flrr, ":"))
			}
		}
	}
	return strings.Join(periodStrs, ", ")
}

func GeoString(gs []goinvestigate.GeoFeatures) string {
	strs := []string{}
	for _, g := range gs {
		score := strconv.FormatFloat(g.VisitRatio, 'f', -1, 64)
		strs = append(strs, strings.Join([]string{g.CountryCode, score}, ":"))
	}
	return strings.Join(strs, ", ")
}

// Appends appendVal to source if cond is true and returns the resulting slice.
// Otherwise, returns source as-is.
func appendIf(source []string, appendVal string, cond bool) []string {
	if cond {
		return append(source, appendVal)
	}
	return source
}

// convenience central wrapper around strconv.FormatFloat(),
// just in case one of these parameters needs to be changed at some point
func convertFloatToStr(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func NewConfig(configFilePath string) (config *Config, err error) {
	tomlFile, err := os.Open(configFilePath)

	if err != nil {
		return nil, err
	}

	tomlData, err := ioutil.ReadAll(tomlFile)

	if err != nil {
		log.Fatal(err)
	}

	if _, err := toml.Decode(string(tomlData), &config); err != nil {
		log.Fatal(err)
	}

	if config.APIKey == "" {
		log.Fatal("Config file is missing APIKey")
	}

	config.NumEndpoints = config.numEndpoints()
	//config.NumFields = config.numFields()

	return config, nil
}

//func (c *Config) numTrueFields() int {
//ctr := 0
////rType := reflect.TypeOf(c)
////rInterfaceVal := reflect.ValueOf(structField)
////rVal := rInterfaceVal.Convert(rType)
//cFields := reflect.ValueOf(c)
//for i := 0; i < rVal.NumField(); i++ {
//if rVal.Field(i).Bool() {
//ctr++
//}
//}
//return ctr
//}

func (c *Config) numEndpoints() int {
	ctr := 0
	if any(c.Categories) || c.Status {
		ctr++
	}
	if any(c.Cooccurrences) {
		ctr++
	}
	if any(c.Related) {
		ctr++
	}
	if any(c.Security) {
		ctr++
	}
	if any(c.TaggingDates) {
		ctr++
	}
	if any(c.DomainRRHistory) {
		ctr++
	}
	return ctr
}

// returns the list of Investiga te functions to call for each domain
func (c *Config) DeriveMessages(inv *goinvestigate.Investigate,
	domain string, respChan chan DomainQueryResponse) (msgs []*DomainQueryMessage) {
	if any(c.Categories) || c.Status {
		msgs = append(msgs, &DomainQueryMessage{
			&CategorizationQuery{
				DomainQuery{inv, domain},
				c.Categories.Labels,
			},
			respChan,
		})
	}
	if any(c.Cooccurrences) {
		msgs = append(msgs, &DomainQueryMessage{
			&CooccurrencesQuery{
				DomainQuery{inv, domain},
			},
			respChan,
		})
	}
	if any(c.Related) {
		msgs = append(msgs, &DomainQueryMessage{
			&RelatedQuery{
				DomainQuery{inv, domain},
			},
			respChan,
		})
	}
	if any(c.Security) {
		msgs = append(msgs, &DomainQueryMessage{
			&SecurityQuery{
				DomainQuery{inv, domain},
			},
			respChan,
		})
	}
	if any(c.TaggingDates) {
		msgs = append(msgs, &DomainQueryMessage{
			&DomainTagsQuery{
				DomainQuery{inv, domain},
			},
			respChan,
		})
	}
	if any(c.DomainRRHistory) {
		msgs = append(msgs, &DomainQueryMessage{
			&DomainTagsQuery{
				DomainQuery{inv, domain},
			},
			respChan,
		})
	}
	return msgs
}

type Config struct {
	APIKey       string
	NumEndpoints int
	//NumFields     int
	Status          bool
	Categories      CategoriesConfig
	Cooccurrences   DomainScoreConfig
	Related         DomainScoreConfig
	Security        SecurityConfig
	TaggingDates    TaggingDatesConfig
	DomainRRHistory DomainRRHistoryConfig
}

type CategoriesConfig struct {
	Labels             bool
	SecurityCategories bool
	ContentCategories  bool
}

type DomainScoreConfig struct {
	Domain bool
	Score  bool
}

type SecurityConfig struct {
	DGAScore               bool
	Perplexity             bool
	Entropy                bool
	SecureRank2            bool
	PageRank               bool
	ASNScore               bool
	PrefixScore            bool
	RIPScore               bool
	Fastflux               bool
	Popularity             bool
	Geodiversity           bool
	GeodiversityNormalized bool
	TLDGeodiversity        bool
	Geoscore               bool
	KSTest                 bool
	Attack                 bool
	ThreatType             bool
}

type TaggingDatesConfig struct {
	Begin    bool
	End      bool
	Category bool
	Url      bool
}

type DomainRRHistoryConfig struct {
	FirstSeen       bool
	LastSeen        bool
	Name            bool
	TTL             bool
	Class           bool
	Type            bool
	RR              bool
	Age             bool
	TTLsMin         bool
	TTLsMax         bool
	TTLsMean        bool
	TTLsMedian      bool
	TTLsStdDev      bool
	CountryCodes    bool
	ASNs            bool
	Prefixes        bool
	RIPSCount       bool
	RIPSDiversity   bool
	Locations       bool
	GeoDistanceSum  bool
	GeoDistanceMean bool
	NonRoutable     bool
	MailExchanger   bool
	CName           bool
	FFCandidate     bool
	RIPSStability   bool
}
