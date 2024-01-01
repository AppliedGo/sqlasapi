/*
<!--
Copyright (c) 2019 Christoph Berger. Some rights reserved.

Use of the text in this file is governed by a Creative Commons Attribution Non-Commercial
Share-Alike License that can be found in the LICENSE.txt file.

Use of the code in this file is governed by a BSD 3-clause license that can be found
in the LICENSE.txt file.

The source code contained in this file may import third-party source code
whose licenses are provided in the respective license files.
-->

<!--
NOTE: The comments in this file are NOT godoc compliant. This is not an oversight.

Comments and code in this file are used for describing and explaining a particular topic to the reader. While this file is a syntactically valid Go source file, its main purpose is to get converted into a blog article. The comments were created for learning and not for code documentation.
-->

+++
title = "SQL as API in Go"
description = "Non-trivial queries can make REST APIs complicated. Valentin Willscher suggests accepting SQL where clauses, with the necessary security checks. Here is how to do it in Go."
author = "Christoph Berger"
email = "chris@appliedgo.net"
date = "2024-01-01"
draft = "false"
categories = ["Internet And Web"]
tags = ["database", "sql", "api"]
articletypes = ["How-To"]
+++

So your API needs to allow queries that are too complicated for plain CRUD APIs but not complicated enough to justify using GraphQL? Consider accepting a subset of SQL `where` clauses, with the necessary security checks implemented in Go.

<!--more-->

What the...?! Exposing SQL via an API endpoint? Not a good idea for sure. But still, for moderately complex queries, simple CRUD APIs are at their limits. If you don't want to switch to full GraphQL, exposing a subset of SQL via a REST API can be a viable alternative.

Valentin Willscher came up with this unconventional idea. In [his blog article][sqlasapi], he explains why exposing SQL where clauses in a controlled manner does not have to be risky and can simplify your API design considerably.

I find this idea really intriguing, but there is a small problem: The sample code is written in Scala. After reading the article, I immediately knew I had to re-implement the code in Go.

## The task: process and validate a specific 'where' clause

Valentin's article describes a particular scenario: a webshop wants to add options for filtering bicycles by features such as weight or material. As an example, consider you want to search for a bicycle made of steel and weighing between 10 and 20 kilograms. A SQL WHERE clause would only need `and`, `between`, and `=` operators:

```sql
material = 'steel' and weight between 10 and 20
```

You could also imagine more complex queries, such as

```sql
(material = 'steel' and weight between 10 and 20) or
(material = 'carbon' and weight between 5 and 15)
```

However, the code I'll unroll below does not implement the OR operation — this is left as an exercise to the reader.

The code's purpose is to ensure that the incoming `where` clauses conform to the expected structure.

## The setup

Do you want to write a full SQL parser? Me too, but not for this purpose. So for the sake of brevity, I'll follow Valentin's article and assume that a third-party SQL parser exits that takes a `where` clause and returns an abstract syntax tree (AST) of that clause.

So I'll start from a data structure that represents this AST. The code I'll unroll below processes and validates this AST and re-creates the original `where` clause. (Valentin's article explains the rationale behind this.)


### Step 1: Define the types

I assume an imaginary SQL parser that returns an AST data structure. The following types model this parser's output. In real life, the SQL parser library would define and export a similar set of types.
*/

// Some imports first
package sqlasapi

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

// `Expr` represents an expression in SQL. It can consist of sub-expressions and values.
type Expr interface {
}

// Operations that are acceptable for the desired `where` clauses must refer to a table column.
type Column struct {
	Name string
}

// The `and` operator has two arbitrary expressions as operands.
type And struct {
	Left, Right Expr
}

// Same for the `or` operator.
type Or struct {
	Left, Right Expr
}

// The `between` operator takes a column name and two integer values.
type Between struct {
	Column       Column
	Lower, Upper int
}

// A parenthesis operation encloses another expression in parentheses.
type Parenthesis struct {
	Expr Expr
}

// The `=` operator shall take a column name and a value. Comparing columns to other columns is not allowed for the `where` clauses modeled here.
type Equals struct {
	Column Column
	Value  Value
}

// Representation of values in SQL expressions
type Value interface {
}

type StringValue struct {
	Value string
}

type IntegerValue struct {
	Value int
}

/*

### Step 2: Parsing the SQL

I'll skip this step because, as mentioned above, I model a simple SQL AST model instead. In a real-world scenario, you'd probably pick a SQL parser library like [`github.com/xwb1989/sqlparser`][sqlparser] to parse the string representation of the SQL query into an AST. For brevity, I assume this is already done.

### Step 3: Process the SQL 'where' expression

Our goal is to allow where clauses that filter for material and weight, using `and`, `between`, and `=` operators. So our imaginary SQL parser would receive a `where` clause like this one...

```sql
(material = 'steel' AND weight BETWEEN 10 AND 20)
```

...and return this AST based on our AST types:

```go
And{
    Left:  Equals{
        Column: Column{
            Name: "material"
        },
        Value: StringValue{
            Value: "steel"
        },
    },
    Right: Between{
        Column: Column{
            Name: "weight"
            },
            Lower: 10,
            Upper: 20
        },
}
```

This is where I start from (as the original article does).

So –

- Our input is a where clause as an AST data structure
- Our output is either
    - a `where` clause in textual format that can be used to query the database, or
    - an error if the input contains disallowed operations or parameters.

Processing the SQL expression works recursively. The function `processSqlExpr()` consists of a type switch statement that works its way through the AST data structure. For every expression that is not a plain value, `processSqlExpr()` calls itself for each of the sub-expressions, then it composes the resulting textual representation from the evaluated expression and its sub-expressions.

For plain values, `processSqlExpr()` calls `processSqlValue()`, which determines if the value is a string or an integer and returns the corresponding textual representation.

If the AST data structure is valid as per our requirements, `processSqlExpr()` returns the sanitized where clause in textual format.

*/
//
func processSqlExpr(expr Expr, columns map[string]struct{}) (string, error) {
	// Inspect the type of the expression.
	switch e := expr.(type) {
	// Check the column name against the whitelist of columns.
	case Column:
		if _, ok := columns[e.Name]; !ok {
			return "", fmt.Errorf("column %s is unknown and not supported", e.Name)
		}
		return e.Name, nil
		// Process the two operands of `and`.
	case And:
		left, err := processSqlExpr(e.Left, columns)
		if err != nil {
			return "", fmt.Errorf("case And -> e.Left: %w", err)
		}
		right, err := processSqlExpr(e.Right, columns)
		if err != nil {
			return "", fmt.Errorf("case And -> e.Right: %w", err)
		}
		// Compose the textual representation.
		return fmt.Sprintf("%s AND %s", left, right), nil
		// "Not implemented yet"
	case Or:
		return "", errors.New("OR clauses are not supported yet")
		// For the `between` operation, process the column name and the two boundaries.
	case Between:
		column, err := processSqlExpr(e.Column, columns)
		if err != nil {
			return "", fmt.Errorf("case Between: %w", err)
		}
		// If required, add further validation for the lower and upper bounds.
		return fmt.Sprintf("%s BETWEEN %d AND %d", column, e.Lower, e.Upper), nil
		// Parentheses pass through the processing unchanged. Only the inner expression is processed.
	case Parenthesis:
		// The only exception: ((double parentheses)). This step de-duplicates them.
		switch e.Expr.(type) {
		case Parenthesis:
			e = e.Expr.(Parenthesis)
		}
		inner, err := processSqlExpr(e.Expr, columns)
		if err != nil {
			return "", fmt.Errorf("case Parenthesis: %w", err)
		}
		return fmt.Sprintf("(%s)", inner), nil
		// The `=` operation must have a column to the left of the `=` operator, and a plain value to the right.
	case Equals:
		column, err := processSqlExpr(e.Column, columns)
		if err != nil {
			return "", fmt.Errorf("case Equals -> e.Column: %w", err)
		}
		value, err := processSqlValue(e.Value)
		if err != nil {
			return "", fmt.Errorf("case Equals -> e.Value: %w", err)
		}
		return fmt.Sprintf("%s = %s", column, value), nil
		// No other expressions are allowed.
	default:
		return "", fmt.Errorf("unsupported expr type: %T", expr)
	}
}

// processSqlValue receives a SQL value and returns the corresponding textual representation.
func processSqlValue(value Value) (string, error) {
	switch v := value.(type) {
	// Strings get quoted.
	case StringValue:
		return fmt.Sprintf("'%s'", v.Value), nil
		// A standard integer-to-string conversion.
	case IntegerValue:
		return strconv.Itoa(v.Value), nil
		// No other types are allowed.
	default:
		return "", fmt.Errorf("unsupported value type: %T", value)
	}
}

/*

### Step 4: Test that thing

A quick, table-based test shows how the SQL processing logic works.

- The code should error out if an `or` statement occurs, which is allowed but not yet implemented.
- The code should verify simple `and`, `between`, and `=` expressions.
- Column names must be from a whitelist.
- Only integer or string values are allowed.

*/

// Run some tests.
func TestProcessSqlExpr(t *testing.T) {
	tests := []struct {
		name    string
		expr    Expr
		columns map[string]struct{}
		want    string
		wantErr bool
	}{
		// The `or` clause is not yet implemented. The code should error out.
		{
			name: "OR clause unsupported",
			expr: Or{
				Left: And{
					Left:  Equals{Column: Column{Name: "material"}, Value: StringValue{Value: "steel"}},
					Right: Between{Column: Column{Name: "weight"}, Lower: 10, Upper: 20},
				},
				Right: And{
					Left:  Equals{Column: Column{Name: "material"}, Value: StringValue{Value: "carbon"}},
					Right: Between{Column: Column{Name: "weight"}, Lower: 5, Upper: 10},
				},
			},
			columns: map[string]struct{}{"material": {}, "weight": {}},
			wantErr: true,
		},
		{
			// A valid `and` expression with parentheses around it should be accepted.
			name: "Nested AND with parentheses",
			expr: Parenthesis{
				Expr: And{
					Left:  Equals{Column: Column{Name: "material"}, Value: StringValue{Value: "steel"}},
					Right: Between{Column: Column{Name: "weight"}, Lower: 10, Upper: 20},
				},
			},
			columns: map[string]struct{}{"material": {}, "weight": {}},
			want:    "(material = 'steel' AND weight BETWEEN 10 AND 20)",
		},
		// If intruders manipulate a column name to retrieve records they are not entitled to retrieve, the code should error out.
		{
			name: "Wrong column name",
			expr: Parenthesis{
				Expr: Parenthesis{
					Expr: And{
						Left:  Equals{Column: Column{Name: "material"}, Value: StringValue{Value: "steel"}},
						Right: Between{Column: Column{Name: "retail_price"}, Lower: 500, Upper: 1000},
					},
				},
			},
			columns: map[string]struct{}{"material": {}, "weight": {}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processSqlExpr(tt.expr, tt.columns)
			if (err != nil) != tt.wantErr {
				t.Errorf("processSqlExpr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("processSqlExpr() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
## Next steps

The above code is the core of the "SQL as API" concept. You'll likely want to add more testing and add security measures beyond checking the query structure and values.

Then, you can build an API that accepts the allowed form of a SQL `where` clause as input, parses it, processes it, runs the generated, sanitized DB query, and returns the results.

## Conclusion

Building a SQL `where` clause validator in Go is quite easy, almost trivial.

No more excuses for writing API's that don't accept SQL queries! No more complex and fragile APIs that attempt to re-invent the SQL wheel.

## How to get and run the code

Option 1: Get the code via git, then cd into the repo and run `go test`.

```sh
git clone https://github.com/appliedgo/sqlasapi
cd sqlasapi
go test -v .
```

Option 2: Run the pure code in the [Go playground][playground].


## Links

- [SQL as API](https://valentin.willscher.de/posts/sql-api/) ([archive](https://web.archive.org/web/20240101200359/https://valentin.willscher.de/posts/sql-api/))
- [The code in the Go Playground](https://go.dev/play/p/MlbmG7OnCrD)

[sqlasapi]: https://valentin.willscher.de/posts/sql-api/
[sqlparser]: https://github.com/xwb1989/sqlparser
[playground]: https://go.dev/play/p/MlbmG7OnCrD

**Happy coding!**

*/
