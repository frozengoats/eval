package eval

import (
	"fmt"
	"strings"
	"testing"

	"github.com/frozengoats/kvstore"
	"github.com/stretchr/testify/assert"
)

var fLookup = func(name string, args ...any) (any, error) {
	switch name {
	case "len":
		if len(args) != 1 {
			return nil, fmt.Errorf("incorrect number of arguments")
		}

		switch t := args[0].(type) {
		case string:
			return len(t), nil
		case []any:
			return len(t), nil
		default:
			return nil, fmt.Errorf("unsupported type")
		}
	case "lines":
		if len(args) != 1 {
			return nil, fmt.Errorf("incorrect number of arguments")
		}

		arg := args[0]
		switch t := arg.(type) {
		case string:
			var res []any
			t = strings.TrimSuffix(t, "\n")
			parts := strings.Split(t, "\n")
			for _, p := range parts {
				res = append(res, p)
			}
			return res, nil
		default:
			return nil, fmt.Errorf("unsupported type %T", t)
		}
	case "strip":
		if len(args) != 1 {
			return nil, fmt.Errorf("incorrect number of arguments")
		}
		switch t := args[0].(type) {
		case string:
			return strings.Trim(t, "\n "), nil
		default:
			return nil, fmt.Errorf("unsupported type %T", t)
		}
	default:
		return nil, fmt.Errorf("unknown function %s", name)
	}
}

func TestGetGroupsSingleQuote(t *testing.T) {
	groups, err := getGroups("hello world && 'hello world'")
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, groups[0].Text, "hello world &&")
	assert.Equal(t, groups[1].Text, "hello world")
}

func TestGetGroupsDoubleQuote(t *testing.T) {
	groups, err := getGroups("hello world && \"hello world\"")
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, groups[0].Text, "hello world &&")
	assert.Equal(t, groups[1].Text, "hello world")
}

func TestGetGroupsParenthesis(t *testing.T) {
	groups, err := getGroups("(hello world) && \"hello world\"")
	assert.NoError(t, err)
	assert.Len(t, groups, 3)
	assert.Equal(t, "hello world", groups[0].Text)
	assert.Equal(t, groups[0].Type, GroupTypeParenthesis)
	assert.Equal(t, "&&", groups[1].Text)
	assert.Equal(t, groups[1].Type, GroupTypeUnqualified)
	assert.Equal(t, "hello world", groups[2].Text)
	assert.Equal(t, groups[2].Type, GroupTypeString)
}

func TestTokenizerSimple(t *testing.T) {
	expression := ".Values.abc.def==123"
	tokenGroup, err := tokenize(expression)
	assert.NoError(t, err)
	assert.Equal(t, TokenTypeGroup, tokenGroup.Type)
	tokens := tokenGroup.Tokens
	assert.Len(t, tokens, 1)

	tokens = tokens[0].Tokens
	assert.Len(t, tokens, 3)
	assert.Equal(t, tokens[0].Type, TokenTypeVariable)
	assert.Equal(t, tokens[1].Type, TokenTypeOperator)
	assert.Equal(t, tokens[2].Type, TokenTypeNumber)
}

func TestTokenizerNestedInQuotes(t *testing.T) {
	expression := ".Values.abc.def==\"(hello==one)\""
	tokenGroup, err := tokenize(expression)
	assert.NoError(t, err)
	assert.Equal(t, TokenTypeGroup, tokenGroup.Type)
	tokens := tokenGroup.Tokens
	assert.Len(t, tokens, 1)

	tokens = tokens[0].Tokens
	assert.Len(t, tokens, 3)
	assert.Equal(t, tokens[0].Type, TokenTypeVariable)
	assert.Equal(t, tokens[1].Type, TokenTypeOperator)
	assert.Equal(t, tokens[2].Type, TokenTypeString)
	assert.Equal(t, tokens[2].Text, "(hello==one)")
}

func TestTokenizerNestedInSingleQuotes(t *testing.T) {
	expression := ".Values.abc.def=='(hello==\"one\")'"
	tokenGroup, err := tokenize(expression)
	assert.NoError(t, err)
	assert.Equal(t, TokenTypeGroup, tokenGroup.Type)
	tokens := tokenGroup.Tokens
	assert.Len(t, tokens, 1)

	tokens = tokens[0].Tokens
	assert.Len(t, tokens, 3)
	assert.Equal(t, TokenTypeVariable, tokens[0].Type)
	assert.Equal(t, TokenTypeOperator, tokens[1].Type)
	assert.Equal(t, TokenTypeString, tokens[2].Type)
	assert.Equal(t, "(hello==\"one\")", tokens[2].Text)
}

func TestTokenizerParenthGroup(t *testing.T) {
	expression := ".Values.ent.value > (.Values.ent2.value || (.Values.ent3.value + 2))"
	tokenGroup, err := tokenize(expression)
	assert.NoError(t, err)
	assert.Equal(t, TokenTypeGroup, tokenGroup.Type)
	tokens := tokenGroup.Tokens
	assert.Len(t, tokens, 1)

	tokens = tokens[0].Tokens
	assert.Len(t, tokens, 3)
	assert.Equal(t, tokens[0].Type, TokenTypeVariable)
	assert.Equal(t, tokens[1].Type, TokenTypeOperator)
	assert.Equal(t, tokens[2].Type, TokenTypeGroup)

	subTok := tokens[2].Tokens
	assert.Len(t, subTok, 3)
	assert.Equal(t, TokenTypeVariable, subTok[0].Type)
	assert.Equal(t, TokenTypeOperator, subTok[1].Type)
	assert.Equal(t, TokenTypeGroup, subTok[2].Type)

	subTok = subTok[2].Tokens
	assert.Len(t, subTok, 1)
	subTok = subTok[0].Tokens

	assert.Equal(t, TokenTypeVariable, subTok[0].Type)
	assert.Equal(t, TokenTypeOperator, subTok[1].Type)
	assert.Equal(t, TokenTypeNumber, subTok[2].Type)
}

func TestEvaluateSimpleExpression(t *testing.T) {
	exp := "strip('  abc def  ') + ' ghi'"
	result, err := Evaluate(exp, nil, fLookup)
	assert.NoError(t, err)
	assert.Equal(t, "abc def ghi", result)
}

func TestEvaluateNestedExpression(t *testing.T) {
	exp := "100 * ((2 + 3) / 5) + 17"
	result, err := Evaluate(exp, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 117.0, result)
}

func TestEvaluateNestedQuotedExpression(t *testing.T) {
	exp := "strip(('  abc ' + ('def ' + 'ghi')) + ' jkl  ')"
	result, err := Evaluate(exp, nil, fLookup)
	assert.NoError(t, err)
	assert.Equal(t, "abc def ghi jkl", result)
}

func TestNestedFuncs(t *testing.T) {
	exp := "len(strip(\" abc def \")) == 2"
	_, err := Evaluate(exp, nil, fLookup)
	assert.NoError(t, err)
}

func TestEvaluateExpressionWithVariables(t *testing.T) {
	values := kvstore.NewStore()
	err := values.Set([]any{100, 101, 102}, "abc", "def")
	assert.NoError(t, err)
	exp := ".abc.def[1]"

	vLookup := func(key string) (any, error) {
		k := strings.TrimPrefix(key, ".")
		return values.Get(kvstore.ParseNamespaceString(k)...), nil
	}

	result, err := Evaluate(exp, vLookup, nil)
	assert.NoError(t, err)
	assert.Equal(t, 101., result)
}

func TestEvaluateExpressionWithNestedVariables(t *testing.T) {
	values := kvstore.NewStore()
	err := values.Set([]any{100, 101, 102}, "abc", "def")
	assert.NoError(t, err)
	exp := "len(.abc.def) + 10"

	vLookup := func(key string) (any, error) {
		k := strings.TrimPrefix(key, ".")
		return values.Get(kvstore.ParseNamespaceString(k)...), nil
	}

	result, err := Evaluate(exp, vLookup, fLookup)
	assert.NoError(t, err)
	assert.Equal(t, 13., result)
}

func TestOrderOfOperations(t *testing.T) {
	exp := "4 + 9 * 9 / 3"
	result, err := Evaluate(exp, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 31., result)
}

func TestIsTruthy(t *testing.T) {
	assert.True(t, IsTruthy(true))
	assert.False(t, IsTruthy(false))
	assert.True(t, IsTruthy(1.))
	assert.False(t, IsTruthy(0.))
	assert.True(t, IsTruthy("hello"))
	assert.False(t, IsTruthy(""))
	assert.False(t, IsTruthy(nil))
	assert.False(t, IsTruthy([]any{}))
	assert.True(t, IsTruthy([]any{1}))
	assert.True(t, IsTruthy(map[string]any{"1": 1}))
	assert.False(t, IsTruthy(map[string]any{}))
}

func TestTemplate(t *testing.T) {
	rendered, err := Template("<! 50 !> hello <! 100 + 2 !> something <! hamster !>", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "50 hello 102 something hamster", rendered)
}

func TestTemplateStartEndEdgeCase(t *testing.T) {
	rendered, err := Template("hello <! 100 * 2 !> something", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "hello 200 something", rendered)
}

func TestTemplateSpace(t *testing.T) {
	rendered, err := Template(" <! 50 !> hello <! 100 + 2 !> something <! hamster !> ", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, " 50 hello 102 something hamster ", rendered)
}

func TestEmptyTemplate(t *testing.T) {
	rendered, err := Template("<!  !>", nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, rendered)
}

func TestIntegerTemplate(t *testing.T) {
	rendered, err := Template("<! 1+1 !>", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2., rendered)
}

func TestNoTemplate(t *testing.T) {
	rendered, err := Template("some perfectly normal text", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "some perfectly normal text", rendered)
}

func TestIterableVariableEvaluation(t *testing.T) {
	values := kvstore.NewStore()
	err := values.Set([]any{100, 101, 102}, "abc", "def")
	assert.NoError(t, err)

	exp := ".abc.def"

	vLookup := func(key string) (any, error) {
		k := strings.TrimPrefix(key, ".")
		return values.Get(kvstore.ParseNamespaceString(k)...), nil
	}

	_, err = Evaluate(exp, vLookup, nil)
	assert.NoError(t, err)
}

func TestBooleanOrderOfOps(t *testing.T) {
	_, err := Evaluate("strip(' abc ') == abc || strip('two') == three || two == two", nil, fLookup)
	assert.NoError(t, err)
}

func TestBooleanOrderOfOpsTwo(t *testing.T) {
	v, err := Evaluate("len(strip(' abc ')) == 3 && len(strip('to')) != 3 && len(strip('two')) == 3", nil, fLookup)
	assert.NoError(t, err)
	vBool, ok := v.(bool)
	assert.True(t, ok)
	assert.True(t, vBool)
}

func TestBooleanOrderOfOpsThree(t *testing.T) {
	kvs := kvstore.NewStore()
	err := kvs.Set("this is a text bundle\nand why not", "stdout")
	assert.NoError(t, err)

	vLookup := func(key string) (any, error) {
		key = strings.TrimPrefix(key, ".")
		v := kvs.Get(kvstore.ParseNamespaceString(key)...)
		return v, nil
	}

	_, err = Evaluate(`len(lines(.stdout)) != 2 || lines(.stdout)[0] != "this is a text bundle"`, vLookup, fLookup)
	assert.NoError(t, err)
}

func TestSubscriptImmediateUnsubscriptable(t *testing.T) {
	_, err := subscriptImmediate("hello", "[2][4][6].abc")
	assert.Error(t, err)
}

func TestSubscriptImmediateSubscriptable(t *testing.T) {
	v, err := subscriptImmediate([]any{"abc", map[string]any{"def": 3}}, "[1].def")
	assert.NoError(t, err)
	assert.Equal(t, 3, v)
}
