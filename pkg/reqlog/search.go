package reqlog

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oklog/ulid"

	"github.com/dstotijn/hetty/pkg/scope"
	"github.com/dstotijn/hetty/pkg/search"
)

var reqLogSearchKeyFns = map[string]func(rl RequestLog) string{
	"req.id":    func(rl RequestLog) string { return rl.ID.String() },
	"req.proto": func(rl RequestLog) string { return rl.Proto },
	"req.url": func(rl RequestLog) string {
		if rl.URL == nil {
			return ""
		}
		return rl.URL.String()
	},
	"req.method":    func(rl RequestLog) string { return rl.Method },
	"req.body":      func(rl RequestLog) string { return string(rl.Body) },
	"req.timestamp": func(rl RequestLog) string { return ulid.Time(rl.ID.Time()).String() },
}

var ResLogSearchKeyFns = map[string]func(rl ResponseLog) string{
	"res.proto":        func(rl ResponseLog) string { return rl.Proto },
	"res.statusCode":   func(rl ResponseLog) string { return strconv.Itoa(rl.StatusCode) },
	"res.statusReason": func(rl ResponseLog) string { return rl.Status },
	"res.body":         func(rl ResponseLog) string { return string(rl.Body) },
}

// TODO: Request and response headers search key functions.

// Matches returns true if the supplied search expression evaluates to true.
func (reqLog RequestLog) Matches(expr search.Expression) (bool, error) {
	switch e := expr.(type) {
	case search.PrefixExpression:
		return reqLog.matchPrefixExpr(e)
	case search.InfixExpression:
		return reqLog.matchInfixExpr(e)
	case search.StringLiteral:
		return reqLog.matchStringLiteral(e)
	default:
		return false, fmt.Errorf("expression type (%T) not supported", expr)
	}
}

func (reqLog RequestLog) matchPrefixExpr(expr search.PrefixExpression) (bool, error) {
	switch expr.Operator {
	case search.TokOpNot:
		match, err := reqLog.Matches(expr.Right)
		if err != nil {
			return false, err
		}

		return !match, nil
	default:
		return false, errors.New("operator is not supported")
	}
}

func (reqLog RequestLog) matchInfixExpr(expr search.InfixExpression) (bool, error) {
	switch expr.Operator {
	case search.TokOpAnd:
		left, err := reqLog.Matches(expr.Left)
		if err != nil {
			return false, err
		}

		right, err := reqLog.Matches(expr.Right)
		if err != nil {
			return false, err
		}

		return left && right, nil
	case search.TokOpOr:
		left, err := reqLog.Matches(expr.Left)
		if err != nil {
			return false, err
		}

		right, err := reqLog.Matches(expr.Right)
		if err != nil {
			return false, err
		}

		return left || right, nil
	}

	left, ok := expr.Left.(search.StringLiteral)
	if !ok {
		return false, errors.New("left operand must be a string literal")
	}

	leftVal := reqLog.getMappedStringLiteral(left.Value)

	if expr.Operator == search.TokOpRe || expr.Operator == search.TokOpNotRe {
		right, ok := expr.Right.(*regexp.Regexp)
		if !ok {
			return false, errors.New("right operand must be a regular expression")
		}

		switch expr.Operator {
		case search.TokOpRe:
			return right.MatchString(leftVal), nil
		case search.TokOpNotRe:
			return !right.MatchString(leftVal), nil
		}
	}

	right, ok := expr.Right.(search.StringLiteral)
	if !ok {
		return false, errors.New("right operand must be a string literal")
	}

	rightVal := reqLog.getMappedStringLiteral(right.Value)

	switch expr.Operator {
	case search.TokOpEq:
		return leftVal == rightVal, nil
	case search.TokOpNotEq:
		return leftVal != rightVal, nil
	case search.TokOpGt:
		// TODO(?) attempt to parse as int.
		return leftVal > rightVal, nil
	case search.TokOpLt:
		// TODO(?) attempt to parse as int.
		return leftVal < rightVal, nil
	case search.TokOpGtEq:
		// TODO(?) attempt to parse as int.
		return leftVal >= rightVal, nil
	case search.TokOpLtEq:
		// TODO(?) attempt to parse as int.
		return leftVal <= rightVal, nil
	default:
		return false, errors.New("unsupported operator")
	}
}

func (reqLog RequestLog) getMappedStringLiteral(s string) string {
	switch {
	case strings.HasPrefix(s, "req."):
		fn, ok := reqLogSearchKeyFns[s]
		if ok {
			return fn(reqLog)
		}
	case strings.HasPrefix(s, "res."):
		if reqLog.Response == nil {
			return ""
		}

		fn, ok := ResLogSearchKeyFns[s]
		if ok {
			return fn(*reqLog.Response)
		}
	}

	return s
}

func (reqLog RequestLog) matchStringLiteral(strLiteral search.StringLiteral) (bool, error) {
	for _, fn := range reqLogSearchKeyFns {
		if strings.Contains(
			strings.ToLower(fn(reqLog)),
			strings.ToLower(strLiteral.Value),
		) {
			return true, nil
		}
	}

	if reqLog.Response != nil {
		for _, fn := range ResLogSearchKeyFns {
			if strings.Contains(
				strings.ToLower(fn(*reqLog.Response)),
				strings.ToLower(strLiteral.Value),
			) {
				return true, nil
			}
		}
	}

	return false, nil
}

func (reqLog RequestLog) MatchScope(s *scope.Scope) bool {
	for _, rule := range s.Rules() {
		if rule.URL != nil && reqLog.URL != nil {
			if matches := rule.URL.MatchString(reqLog.URL.String()); matches {
				return true
			}
		}

		for key, values := range reqLog.Header {
			var keyMatches, valueMatches bool

			if rule.Header.Key != nil {
				if matches := rule.Header.Key.MatchString(key); matches {
					keyMatches = true
				}
			}

			if rule.Header.Value != nil {
				for _, value := range values {
					if matches := rule.Header.Value.MatchString(value); matches {
						valueMatches = true
						break
					}
				}
			}
			// When only key or value is set, match on whatever is set.
			// When both are set, both must match.
			switch {
			case rule.Header.Key != nil && rule.Header.Value == nil && keyMatches:
				return true
			case rule.Header.Key == nil && rule.Header.Value != nil && valueMatches:
				return true
			case rule.Header.Key != nil && rule.Header.Value != nil && keyMatches && valueMatches:
				return true
			}
		}

		if rule.Body != nil {
			if matches := rule.Body.Match(reqLog.Body); matches {
				return true
			}
		}
	}

	return false
}
