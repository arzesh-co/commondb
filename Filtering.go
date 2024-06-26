package CommenDb

import (
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strconv"
)

type filter struct {
	Condition any
	Label     string
	Operation string
}
type aggregation struct {
	GroupBy      string `json:"group_by"`
	GroupByTitle string `json:"group_by_title"`
	Aggregators  []struct {
		Aggregate string `json:"aggregate"`
		Operation string `json:"operation"`
	} `json:"aggregators"`
}
type sort struct {
	DbName string `json:"db_name"`
	Type   string `json:"type"`
}
type MongoPipeLine struct {
	Filter      map[string]any
	Sort        map[string]any
	Aggregation map[string]any
	Skip        string
	Limit       string
	Fields      map[string]int8
}

func createFilter(cond filter) interface{} {
	switch cond.Operation {
	case "text":
		return bson.M{"$text": bson.M{"$search": cond.Condition.(string)}}
	case "Start With":
		return primitive.Regex{Pattern: "^" + cond.Condition.(string) + ".", Options: "i"}
	case "End With":
		return primitive.Regex{Pattern: ".*" + cond.Condition.(string) + "$", Options: "i"}
	case "Equal":
		return bson.M{"$eq": cond.Condition}
	case "Include":
		return primitive.Regex{Pattern: ".*" + cond.Condition.(string) + ".*", Options: "i"}
	case "Empty":
		return bson.M{"$exists": false}
	case "not Empty":
		return bson.M{"$exists": true}
	case "=":
		if fmt.Sprintf("%T", cond.Condition) == "[]interface {}" {
			return bson.M{"$in": cond.Condition}
		}
		return bson.M{"$eq": ConvertFilterCondition(cond.Condition)}
	case ">=":
		return bson.M{"$gte": ConvertFilterCondition(cond.Condition)}
	case "<=":
		return bson.M{"$lte": ConvertFilterCondition(cond.Condition)}
	case ">":
		return bson.M{"$gt": ConvertFilterCondition(cond.Condition)}
	case "<":
		return bson.M{"$lt": ConvertFilterCondition(cond.Condition)}
	case "!=":
		return bson.M{"$ne": ConvertFilterCondition(cond.Condition)}
	}
	return bson.M{}
}
func ConvertFilterCondition(condition any) any {
	switch condition.(type) {
	case string:
		switch ConvertorType(condition.(string)) {
		case "func":
			return findFunc(condition.(string))
		case "string":
			return condition
		default:
			return condition
		}
	default:
		return condition
	}
}
func CreateAggregation(aggr string) map[string]interface{} {
	agg := &aggregation{}
	err := json.Unmarshal([]byte(aggr), agg)
	if err != nil {
		return nil
	}
	filter := make(map[string]interface{})
	if aggr == "" {
		return nil
	}
	filter["_id"] = "$" + agg.GroupBy
	filter["group_by_title"] = bson.M{"$first": "$" + agg.GroupByTitle}
	for _, aggregator := range agg.Aggregators {
		switch aggregator.Operation {
		case "avg":
			filter[aggregator.Aggregate] = bson.M{"$avg": "$" + aggregator.Aggregate}
		case "sum":
			filter[aggregator.Aggregate] = bson.M{"$sum": "$" + aggregator.Aggregate}
		case "count":
			filter[aggregator.Aggregate] = bson.M{"$sum": 1}
		case "min":
			filter[aggregator.Aggregate] = bson.M{"$min": "$" + aggregator.Aggregate}
		case "max":
			filter[aggregator.Aggregate] = bson.M{"$max": "$" + aggregator.Aggregate}
		case "first":
			filter[aggregator.Aggregate] = bson.M{"$first": "$" + aggregator.Aggregate}
		}
	}
	return filter
}
func CreateFilter(filterString string) (map[string]any, error) {
	var filters []filter
	err := json.Unmarshal([]byte(filterString), &filters)
	if err != nil {
		return nil, err
	}
	clintFilterMap := make(map[string]any)
	if len(filters) > 1 {
		listCondition := make([]map[string]any, len(filters))
		for i, f := range filters {
			condition := make(map[string]any)
			condition[f.Label] = createFilter(f)
			listCondition[i] = condition
		}
		clintFilterMap["$and"] = listCondition
		return clintFilterMap, nil
	}
	clintFilterMap[filters[0].Label] = createFilter(filters[0])
	return clintFilterMap, nil
}
func CreateSorting(sortString string) (map[string]any, error) {
	var sorts []sort
	err := json.Unmarshal([]byte(sortString), &sorts)
	if err != nil {
		return nil, err
	}
	sortFilter := make(map[string]any)
	for _, s := range sorts {
		switch s.Type {
		case "asc":
			sortFilter[s.DbName] = 1
		case "des":
			sortFilter[s.DbName] = -1
		}
	}
	return sortFilter, nil
}
func CreatePipeLineMongoAggregate(funcFilter bson.M, line *MongoPipeLine) ([]bson.M, any) {
	var filterCount interface{}
	Skip, err := strconv.ParseInt(line.Skip, 10, 64)
	if err != nil {
		Skip = 0
	}
	Limit, err := strconv.ParseInt(line.Limit, 10, 64)
	if err != nil {
		Limit = 10
	}
	if line.Filter != nil {
		filterCount = bson.M{"$and": []interface{}{funcFilter, line.Filter}}
	} else {
		filterCount = funcFilter
	}
	var skipPage int64
	if Skip != 0 {
		skipPage = (Skip - 1) * Limit
	} else {
		skipPage = 0
	}
	pipe := []bson.M{{"$match": funcFilter}}
	if len(line.Filter) != 0 {
		pipe = append(pipe, bson.M{"$match": line.Filter})
	}
	if len(line.Fields) != 0 {
		pipe = append(pipe, bson.M{"$project": line.Fields})
	}
	if len(line.Aggregation) != 0 {
		pipe = append(pipe, bson.M{"$group": line.Aggregation})
	}
	if len(line.Sort) != 0 {
		pipe = append(pipe, bson.M{"$sort": line.Sort})
	}
	pipe = append(pipe, bson.M{"$skip": skipPage})
	pipe = append(pipe, bson.M{"$limit": Limit})
	return pipe, filterCount
}
