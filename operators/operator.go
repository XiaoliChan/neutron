package operators

import "github.com/chainreactors/parsers/iutils"

// operators contains the operators that can be applied on protocols
type Operators struct {
	// Matchers contains the detection mechanism for the request to identify
	// whether the request was successful
	Matchers []*Matcher `json:"matchers,omitempty" yaml:"matchers,omitempty"`
	// Extractors contains the extraction mechanism for the request to identify
	// and extract parts of the response.
	Extractors []*Extractor `json:"extractors,omitempty" yaml:"extractors,omitempty"`
	// MatchersCondition is the condition of the matchers
	// whether to use AND or OR. Default is OR.
	MatchersCondition string `json:"matchers-condition,omitempty" yaml:"matchers-condition,omitempty"`
	// cached variables that may be used along with request.
	matchersCondition ConditionType

	// TemplateID is the ID of the template for matcher
	TemplateID string
}

// Result is a result structure created from operators running on data.
type Result struct {
	// Matched is true if any matchers matched
	Matched bool
	// Extracted is true if any result type values were extracted
	Extracted bool
	// Matches is a map of matcher names that we matched
	Matches map[string][]string
	// Extracts contains all the data extracted from inputs
	Extracts map[string][]string
	// OutputExtracts is the list of extracts to be displayed on screen.
	OutputExtracts []string
	// DynamicValues contains any dynamic values to be templated
	DynamicValues map[string]interface{}
	// PayloadValues contains payload values provided by user. (Optional)
	PayloadValues map[string]interface{}
}

func (r *Operators) Compile() error {
	if r.MatchersCondition != "" {
		r.matchersCondition = conditionTypes[r.MatchersCondition]
	} else {
		r.matchersCondition = OrCondition
	}
	for _, matcher := range r.Matchers {
		if err := matcher.CompileMatchers(); err != nil {
			return err
		}
	}
	for _, extractor := range r.Extractors {
		if err := extractor.CompileExtractors(); err != nil {
			return err
		}
	}
	return nil
}

// getMatchersCondition returns the condition for the matchers
func (r *Operators) GetMatchersCondition() ConditionType {
	return r.matchersCondition
}

type matchFunc func(data map[string]interface{}, matcher *Matcher) bool
type extractFunc func(data map[string]interface{}, matcher *Extractor) map[string]struct{}

// Execute executes the operators on data and returns a result structure
func (operators *Operators) Execute(data map[string]interface{}, match matchFunc, extract extractFunc) (*Result, bool) {
	matcherCondition := operators.GetMatchersCondition()

	var matches bool
	result := &Result{
		Matches:       make(map[string][]string),
		Extracts:      make(map[string][]string),
		DynamicValues: make(map[string]interface{}),
	}

	//// Start with the extractors first and evaluate them.
	var tmpname int
	for _, extractor := range operators.Extractors {
		var extractorResults []string

		for match := range extract(data, extractor) {
			extractorResults = append(extractorResults, match)

			if extractor.Internal {
				if _, ok := result.DynamicValues[extractor.Name]; !ok {
					result.DynamicValues[extractor.Name] = match
				}
			} else {
				result.OutputExtracts = append(result.OutputExtracts, match)
			}
		}
		if len(extractorResults) > 0 && !extractor.Internal {
			if extractor.Name != "" {
				result.Extracts[extractor.Name] = extractorResults
			} else {
				result.Extracts[iutils.ToString(tmpname)] = extractorResults
				tmpname++
			}
		}
	}

	for _, matcher := range operators.Matchers {
		// Check if the matcher matched
		if !match(data, matcher) {
			// If the condition is AND we haven't matched, try next request.
			if matcherCondition == AndCondition {
				//if len(result.DynamicValues) > 0 {
				//	return result, true
				//}
				return nil, false
			}
		} else {
			// If the matcher has matched, and its an OR
			// write the first output then move to next matcher.
			if matcherCondition == OrCondition && matcher.Name != "" {
				result.Matches[matcher.Name] = []string{}
			}
			matches = true
		}
	}

	result.Matched = matches
	result.Extracted = len(result.OutputExtracts) > 0
	if len(result.DynamicValues) > 0 {
		return result, true
	}
	// Don't print if we have matchers and they have not matched, irregardless of extractor
	if len(operators.Matchers) > 0 && !matches {
		return nil, false
	}
	// Write a final string of output if matcher type is
	// AND or if we have extractors for the mechanism too.
	if len(result.Extracts) > 0 || len(result.OutputExtracts) > 0 || matches {
		return result, true
	}

	return nil, true
}

// ExecuteInternalExtractors executes internal dynamic extractors
func (operators *Operators) ExecuteInternalExtractors(data map[string]interface{}, extract extractFunc) map[string]interface{} {
	dynamicValues := make(map[string]interface{})

	// Start with the extractors first and evaluate them.
	for _, extractor := range operators.Extractors {
		if !extractor.Internal {
			continue
		}
		for match := range extract(data, extractor) {
			if _, ok := dynamicValues[extractor.Name]; !ok {
				dynamicValues[extractor.Name] = match
			}
		}
	}
	return dynamicValues
}

// MakeDynamicValuesCallback takes an input dynamic values map and calls
// the callback function with all variations of the data in input in form
// of map[string]string (interface{}).
func MakeDynamicValuesCallback(input map[string][]string, iterateAllValues bool, callback func(map[string]interface{}) bool) {
	output := make(map[string]interface{}, len(input))

	if !iterateAllValues {
		for k, v := range input {
			if len(v) > 0 {
				output[k] = v[0]
			}
		}
		callback(output)
		return
	}
	inputIndex := make(map[string]int, len(input))

	var maxValue int
	for _, v := range input {
		if len(v) > maxValue {
			maxValue = len(v)
		}
	}

	for i := 0; i < maxValue; i++ {
		for k, v := range input {
			if len(v) == 0 {
				continue
			}
			if len(v) == 1 {
				output[k] = v[0]
				continue
			}
			if gotIndex, ok := inputIndex[k]; !ok {
				inputIndex[k] = 0
				output[k] = v[0]
			} else {
				newIndex := gotIndex + 1
				if newIndex >= len(v) {
					output[k] = v[len(v)-1]
					continue
				}
				output[k] = v[newIndex]
				inputIndex[k] = newIndex
			}
		}
		// skip if the callback says so
		if callback(output) {
			return
		}
	}
}
