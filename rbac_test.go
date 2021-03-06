package main

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func getTestContext(path string, body string, method string) (*requestContext, error) {
	req, _ := http.NewRequest(method, "http://localhost:9200"+path, bytes.NewBufferString(body))
	req.Header.Add("X-Remote-User", "dustind")
	req.Header.Add("X-Remote-Groups", "OU=thing,CN=group2,DC=something")

	var c Config
	c.getConf("config.example.yaml")

	var trace Trace

	ctx, err := getRequestContext(req, &c, &trace)

	return ctx, err
}

func TestIndexPermitted(t *testing.T) {
	body := `{"index":"*","ignore":[404],"timeout":"90s","requestTimeout":90000,"ignoreUnavailable":true}
{"size":0,"query":{"bool":{"must":[{"range":{"@timestamp":{"gte":1519223869113,"lte":1519225669114,"format":"epoch_millis"}}},{"bool":{"must":[{"match_all":{}}],"must_not":[]}}]}},"aggs":{"61ca57f1-469d-11e7-af02-69e470af7417":{"filter":{"match_all":{}},"aggs":{"timeseries":{"date_histogram":{"field":"@timestamp","interval":"30s","min_doc_count":0,"time_zone":"America/Chicago","extended_bounds":{"min":1519223869113,"max":1519225669114}},"aggs":{"61ca57f2-469d-11e7-af02-69e470af7417":{"bucket_script":{"buckets_path":{"count":"_count"},"script":{"inline":"count * 1","lang":"expression"},"gap_policy":"skip"}}}}}}}}
`

	ctx, err := getTestContext("/*/_search", body, "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}

	ok, err := indexPermitted(ctx)
	if ok == false || err != nil {
		t.Error("index not permitted or err: ", err)
	}
}

func TestIndexNotPermitted(t *testing.T) {
	ctx, err := getTestContext("/secret_stuff/_search", "", "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}

	ok, err := indexPermitted(ctx)
	if ok == true || err != nil {
		t.Error("index permitted or err: ", err)
	}
}

func TestAPIPermitted(t *testing.T) {
	// GET to _nodes
	ctx, err := getTestContext("/_nodes/local", "", "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}
	ok, err := apiPermitted(ctx)
	if ok == false || err != nil {
		t.Error("API not permitted or err: ", err)
	}

	// PUT to _template
	ctx, err = getTestContext("/_template/kibana_index_template", "", "PUT")
	if err != nil {
		t.Error("could not get context: ", err)
	}
	if extractAPI(ctx.r) != "_template" {
		t.Error("Expected _template for API, got: ", extractAPI(ctx.r))
	}
	ok, err = apiPermitted(ctx)
	if ok == false || err != nil {
		t.Error("API not permitted or err: ", err)
	}

	// DELETE to _template
	ctx, err = getTestContext("/_template/kibana_index_template", "", "DELETE")
	if err != nil {
		t.Error("could not get context: ", err)
	}
	ok, err = apiPermitted(ctx)
	if ok == true || err != nil {
		t.Error("API permitted or err: ", err)
	}
}

func TestAPINotPermitted(t *testing.T) {
	ctx, err := getTestContext("/test_deflek/_settings", "", "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}

	ok, err := apiPermitted(ctx)
	if ok || err != nil {
		t.Error("API permitted or err: ", err)
	}
}

func TestGetWhitelistedIndices(t *testing.T) {

	expectedIndices := []Index{
		Index{Name: "test_deflek",
			RESTverbs: []string{
				"GET", "POST",
			}},
		Index{Name: "test_deflek2",
			RESTverbs: []string{
				"GET",
			}},
		Index{Name: "globby-*",
			RESTverbs: []string{
				"GET",
			}},
		Index{Name: ".kibana",
			RESTverbs: []string{
				"GET", "POST",
			}},
	}

	ctx, err := getTestContext("/", "", "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}

	extractedIndices, err := getWhitelistedIndices(ctx.r, ctx.C)
	if err != nil {
		t.Error("got error while getting whitelisted indices: ", err)
	}

	if diff := cmp.Diff(expectedIndices, extractedIndices); diff != "" {
		t.Errorf("unexpected difference: (-got +want)\n%s", diff)
	}
}

func TestGetUser(t *testing.T) {
	ctx, err := getTestContext("/", "", "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}
	user, err := getUser(ctx.r, ctx.C)
	if err != nil {
		t.Error("could not get user: ", err)
	}

	if user != "dustind" {
		t.Errorf("got %s, expected %s", user, "dustind")

	}
}

func TestCheckRBAC(t *testing.T) {
	body := `{"index":"*","ignore":[404],"timeout":"90s","requestTimeout":90000,"ignoreUnavailable":true}
{"size":0,"query":{"bool":{"must":[{"range":{"@timestamp":{"gte":1519223869113,"lte":1519225669114,"format":"epoch_millis"}}},{"bool":{"must":[{"match_all":{}}],"must_not":[]}}]}},"aggs":{"61ca57f1-469d-11e7-af02-69e470af7417":{"filter":{"match_all":{}},"aggs":{"timeseries":{"date_histogram":{"field":"@timestamp","interval":"30s","min_doc_count":0,"time_zone":"America/Chicago","extended_bounds":{"min":1519223869113,"max":1519225669114}},"aggs":{"61ca57f2-469d-11e7-af02-69e470af7417":{"bucket_script":{"buckets_path":{"count":"_count"},"script":{"inline":"count * 1","lang":"expression"},"gap_policy":"skip"}}}}}}}}
`

	ctx, err := getTestContext("/*/_search", body, "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}

	var p Prox

	ok, err := p.checkRBAC(ctx)
	if !ok || err != nil {
		t.Error("index not permitted or err: ", err)
	}
}

func TestCanManage(t *testing.T) {
	ctx, err := getTestContext("/foo", "", "GET")
	if err != nil {
		t.Error("could not get context: ", err)
	}

	ok, err := canManage(ctx.r, ctx.C)
	if !ok || err != nil {
		t.Error("should be able to manage or error but got: ", ok)
	}
}

func indexInSlice(a Index, indices []Index) bool {
	for _, b := range indices {
		if b.Name == a.Name {
			return true
		}
	}
	return false
}
