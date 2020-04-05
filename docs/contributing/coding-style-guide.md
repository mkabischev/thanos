---
title: Coding Style Guide
type: docs
menu: contributing
---

# Thanos Coding Style Guide

This document details the official style guides for the various languages we use in the Thanos project.
Feel free to familiarize yourself with and refer to this document during code reviews. If something in our codebase does not match the style, it means it
was missed or it was written before this document. Help wanted to fix it! (:

Generally we care about:

* Readability, so low [Cognitive Load](https://www.dabapps.com/blog/cognitive-load-programming/).
* Maintainability. We avoid the code that **surprises**.
* Performance only for critical path and without compromising readability.
* Testability. Even if it means some changes to the production code, like `timeNow func() time.Time` mock.
* Consistency: If some pattern repeats, it means fewer surprises.

Some style is enforced by our linters and is covered in separate smaller sections. Please look there if you want to
embrace some of the rules in your own project! For Thanos developers we recommend to read sections about rules to manually apply during
development. Some of those are currently impossible to detect with linters. Ideally everything would be automated. (:

## TOC

- [Thanos Coding Style Guide](#thanos-coding-style-guide)
  * [TOC](#toc)
- [Go](#go)
  * [Development / Code Review](#development---code-review)
    + [Reliability](#reliability)
      - [Defers: Don't Forget to Check Returned Errors](#defers--don-t-forget-to-check-returned-errors)
      - [Exhaust Readers](#exhaust-readers)
      - [Avoid Globals](#avoid-globals)
      - [Never Use Panics](#never-use-panics)
      - [Avoid Using the `reflect` or `unsafe` Packages](#avoid-using-the--reflect--or--unsafe--packages)
    + [Performance](#performance)
      - [Pre-allocating Slices and Maps](#pre-allocating-slices-and-maps)
      - [Reuse arrays](#reuse-arrays)
    + [Readability](#readability)
      - [Only Two Ways of Formatting Functions/Methods](#only-two-ways-of-formatting-functions-methods)
      - [Control Structure: Avoid `else`](#control-structure--avoid--else-)
      - [Prints.](#prints)
      - [Use Named Return Parameters Carefully](#use-named-return-parameters-carefully)
      - [Explicitly Handled Return Errors](#explicitly-handled-return-errors)
      - [Wrap Errors for More Context; Don't Repeat "failed ..." There.](#wrap-errors-for-more-context--don-t-repeat--failed---there)
      - [Use the Blank Identifier `_`](#use-the-blank-identifier----)
    + [Testing](#testing)
      - [Table Tests](#table-tests)
      - [Tests for Packages / Structs That Involve `time` package.](#tests-for-packages---structs-that-involve--time--package)
  * [Ensured by linters](#ensured-by-linters)
- [Bash](#bash)

<small><i>Table of contents generated with <a href='http://ecotrust-canada.github.io/markdown-toc/'>markdown-toc</a></i></small>

# Go

For code written in [Go](https://golang.org/) we use the standard Go style guides ([Effective Go](https://golang.org/doc/effective_go.html),
[CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)) with a few additional rules that make certain areas stricter
than the standard guides. This ensures even better consistency in modern distributed system databases like Thanos, where reliability, performance and
maintainability are extremely important. See more rationales behind some of the rules in [bwplotka's blog post](todo).

<img src="../img/go-in-thanos.jpg" class="img-fluid" alt="Go in Thanos" />

<!--
Things skipped, since covered in Effective Go or CodeReviewsComments already:
*

TODO still:
* log lowercase
* errors.Cause
* variable shadowing, package name shadowing
* ever do this on an exported type. If you embed sync.Mutex that makes "Lock" and "Unlock" part of your exported interface. Now the caller of your API doesn't know if they are supposed to call Lock/Unlock, or if this is a pure internal implementation detail.
* Comment on surprises.
* Avoid configs vs arguments
* Unsafe convert

NOTE: Because of blackfriday bug, we have to change those code ` snippet to < highlight go > hugo shortcodes during `websitepreprocessing.sh` for website.
-->

## Development / Code Review

In this section we will go through rules that on top of the standard guides that we apply during development and code reviews.

NOTE: If you know that any of those rules can be enabled by some linter, automatically, let us know! (:

### Reliability

#### Defers: Don't Forget to Check Returned Errors

It's easy to forget to check the error returned by a `Close` method that we deferred.

```go
f, err := os.Open(...)
if err != nil {
    // handle..
}
defer f.Close() // What if an error occurs here?

// Write something to file... etc.
```

Unchecked errors like this can lead to major bugs. Consider the above example: the `*os.File` `Close` method can be responsible
for actually flushing to the file, so if error occurs at that point, the whole write might be aborted! 😱

Always check errors! To make it consistent and not distracting, use our [runutil](https://pkg.go.dev/github.com/thanos-io/thanos@v0.11.0/pkg/runutil?tab=doc)
helper package, e.g.:

```go
// Use `CloseWithErrCapture` if you want to close and fail the function or
// method on a `f.Close` error (make sure thr `error` return argument is
// named as `err`). If the error is already present, `CloseWithErrCapture`
// will append (not wrap) the `f.Close` error if any.
defer runutil.CloseWithErrCapture(&err, f, "close file")

// Use `CloseWithLogOnErr` if you want to close and log error on `Warn`
// level on a `f.Close` error.
defer runutil.CloseWithLogOnErr(logger, f, "close file")
```

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
func writeToFile(...) error {
    f, err := os.Open(...)
    if err != nil {
        return err
    }
    defer f.Close() // What if an error occurs here?

    // Write something to file...
    return nil
}
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
func writeToFile(...) (err error) {
    f, err := os.Open(...)
    if err != nil {
        return err
    }
    // Now all is handled well.
    defer runutil.CloseWithErrCapture(&err, f, "close file")

    // Write something to file...
    return nil
}
```

</td></tr>
</tbody></table>

#### Exhaust Readers

One of the most common bugs is forgetting to close or fully read the bodies of HTTP requests and responses, especially on
error. If you read body of such structures, you can use the [runutil](https://pkg.go.dev/github.com/thanos-io/thanos@v0.11.0/pkg/runutil?tab=doc)
helper as well:

```go
defer runutil.ExhaustCloseWithLogOnErr(logger, resp.Body, "close response")
```

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
resp, err := http.Get("http://example.com/")
if err != nil {
    // handle...
}
defer runutil.CloseWithLogOnErr(logger, resp.Body, "close response")

scanner := bufio.NewScanner(resp.Body)
// If any error happens and we return in the middle of scanning
// body, we can end up with unread buffer, which
// will use memory and hold TCP connection!
for scanner.Scan() {
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
resp, err := http.Get("http://example.com/")
if err != nil {
    // handle...
}
defer runutil.ExhaustCloseWithLogOnErr(logger, resp.Body, "close response")

scanner := bufio.NewScanner(resp.Body)
// If any error happens and we return in the middle of scanning body,
// defer will handle all well.
for scanner.Scan() {
```

</td></tr>
</tbody></table>

#### Avoid Globals

No globals other than `const` are allowed. Period.
This means also, no `init` functions.

#### Never Use Panics

Never use them. If some dependency use it, use [recover](https://golang.org/doc/effective_go.html#recover). Also consider
avoiding that dependency. 🙈

#### Avoid Using the `reflect` or `unsafe` Packages

Use those only for very specific, critical cases. Especially `reflect` tend to be be very slow. For testing code it's fine to use reflect.

### Performance

After all, Thanos system is a database that has to perform queries over terabytes of data within human friendly response times.
This requires some additional patterns to our code. With those patterns try to not sacrifice the readability and apply those only
on critical code paths. Also always measure. The Go performance relies on many hidden things, so the good micro benchmark, following
with the real system load test is the key.

#### Pre-allocating Slices and Maps

Try to always preallocate slices and map either via `cap` or `length`. If you know the number elements you want to put
apriori, use that knowledge!  This significantly improves the latency of such code. Consider this as micro optimization,
however it's a good pattern to do it always, as it does not add much complexity. Performance wise, it's only relevant for critical,
code paths with big arrays.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
func copyIntoSliceAndMap(biggy []string) (a []string, b map[string]struct{})
    b = map[string]struct{}{}

    for _, item := range biggy {
        a = append(a, item)
        b[item] = struct{}
    }
}
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
func copyIntoSliceAndMap(biggy []string) (a []string, b map[string]struct{})
    b = make(map[string]struct{}, len(biggy))
    a = make([]string, len(biggy))

    // Copy will not even work without pre-allocation.
    copy(a, biggy)
    for _, item := range biggy {
        b[item] = struct{}
    }
}
```

</td></tr>
</tbody></table>

#### Reuse arrays

To extend above point, there are cases where you don't need to allocate anything all the time. If you repeat certain operation on slices
sequentially, it's reasonable to reuse underlying array for those. This can give quite enormous gains for critical paths.
Unfortunately there is no way to reuse underlying array for maps.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
var messages []string{}
for _, msg := range recv {
    messages = append(messages, msg)

    if len(messages) > maxMessageLen {
        marshalAndSend(messages)
        // This creates new array. Previous array
        // will be garbage collected only after
        // some time (seconds), which
        // can create enormous memory pressure.
        messages = []string{}
    }
}
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
var messages []string{}
for _, msg := range recv {
    messages = append(messages, msg)

    if len(messages) > maxMessageLen {
        marshalAndSend(messages)
        // Instead of new array, reuse
        // the same, with the same capacity,
        // just length equals to zero.
        messages = messages[:0]
    }
}
```

</td></tr>
</tbody></table>

### Readability

#### Only Two Ways of Formatting Functions/Methods

Prefer function/method definitions with arguments in a single line. If it's too wide, put each argument on a new line.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
func function(argument1 int, argument2 string,
    argument3 time.Duration, argument4 someType,
    argument5 float64, argument6 time.Time,
) (ret int, err error) {
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
func function(
    argument1 int,
    argument2 string,
    argument3 time.Duration,
    argument4 someType,
    argument5 float64,
    argument6 time.Time,
) (ret int, err error)
```

</td></tr>
</tbody></table>

#### Control Structure: Avoid `else`

In most of the cases you don't need `else`. You can usually use `continue`, `break` or `return` to end an `if` block.
This enables having one less indent and netter consistency so code is more readable.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
for _, elem := range elems {
    if a == 1 {
        something[i] = "yes"
    } else
        something[i] = "no"
    }
}
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
for _, elem := range elems {
    if a == 1 {
        something[i] = "yes"
        continue
    }
    something[i] = "no"
}
```

</td></tr>
</tbody></table>

#### Use Named Return Parameters Carefully

It's OK to name return parameters if the types do not give enough information about what function or method actually returns.
Another use case is when you want to define a variable, e.g. a slice.

**IMPORTANT:** never use naked `return` statements with named return perameters. This compiles but it makes returning values
implicit and thus more prone to surprises.

#### Explicitly Handled Return Errors

Always address returned errors. It does not mean you cannot "ignore" the error for some reason, e.g. if we know implementation
will not return anything meaningful. You can ignore the error, but do so explicitly:

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
someMethodThatReturnsError(...)
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>


```go
_ = someMethodThatReturnsError(...)
```

</td></tr>
</tbody></table>

The exception: well known cases such as `level.Debug|Warn` etc and `fmt.FPrint*`

#### Wrap Errors for More Context; Don't Repeat "failed ..." There.

We use [`pkg/errors`](https://github.com/pkg/errors) package for `errors`. We prefer it over standard wrapping with `fmt.Sprintf` + `%w`,
as `errors.Wrap` is explicit. It's easy to by accident replace `%w` with `%v` or to add extra inconsistent characters to the string.

Use [`pkg/errors.Wrap`](https://github.com/pkg/errors) to wrap errors for future context when errors occurs. It's recommended
to add more interesting variables to add context using `errors.Wrapf`, e.g. file names, IDs or things that fail, etc.

NOTE: never prefix wrap messages with wording like `failed ... ` or `error occurred while...`. Just describe what we
wanted to do when the failure occurred. Those prefixes are just noise. We are wrapping error, so it's obvious that some error
occurred, right? (: Improve readability and consider avoiding those.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
if err != nil {
    return errors.Wrapf(err, "error while reading from file %s", f.Name)
}
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
if err != nil {
    return errors.Wrapf(err, "read file %s", f.Name)
}
```

</td></tr>
</tbody></table>

#### Use the Blank Identifier `_`

Black identifiers are very useful to mark variables that are not used. Consider the following cases:

```go
// We don't need the second return parameter.
// Let's use the blank identifier instead.
a, _, err := function1(...)```

```go
// We don't need to use this variable, we
// just want to make sure TypeA implements InterfaceA.
var _ InterfaceA = TypeA
```

```go
// We don't use context argument; let's use the blank
// identifier to make it clear.
func (t *Type) SomeMethod(_ context.Context, abc int) error {
```

### Testing

#### Table Tests

Use table-driven tests that use [t.Run](https://blog.golang.org/subtests) for readability. They are easy to read
and allows to add clean description of each test case. Adding or adapting test cases is also easier.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>

```go
host, port, err := net.SplitHostPort("1.2.3.4:1234")
testutil.Ok(t, err)
testutil.Equals(t, "1.2.3.4", host)
testutil.Equals(t, "1234", port)

host, port, err = net.SplitHostPort("1.2.3.4:something")
testutil.Ok(t, err)
testutil.Equals(t, "1.2.3.4", host)
testutil.Equals(t, "http", port)

host, port, err = net.SplitHostPort(":1234")
testutil.Ok(t, err)
testutil.Equals(t, "", host)
testutil.Equals(t, "1234", port)

host, port, err = net.SplitHostPort("yolo")
testutil.NotOk(t, err)
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>

```go
for _, tcase := range []struct{
    name string

    input     string

    expectedHost string
    expectedPort string
    expectedErr error
}{
    {
        name: "host and port",

        input:     "1.2.3.4:1234",
        expectedHost: "1.2.3.4",
        expectedPort: "1234",
    },
    {
        name: "host and named port",

        input:     "1.2.3.4:something",
        expectedHost: "1.2.3.4",
        expectedPort: "something",
    },
    {
        name: "just port",

        input:     ":1234",
        expectedHost: "",
        expectedPort: "1234",
    },
    {
        name: "not valid hostport",

        input:     "yolo",
        expectedErr: errors.New("<exact error>")
    },
}{
    t.Run(tcase.name, func(t *testing.T) {
        host, port, err := net.SplitHostPort(tcase.input)
        if tcase.expectedErr != nil {
            testutil.NotOk(t, err)
            testutil.Equals(t, tcase.expectedErr, err)
            return
        }
        testutil.Ok(t, err)
        testutil.Equals(t, tcase.expectedHost, host)
        testutil.Equals(t, tcase.expectedPort, port)
    })
}
```

</td></tr>
</tbody></table>

#### Tests for Packages / Structs That Involve `time` package.

Avoid unit testing based on real time. Always try to mock time that is used within struct by using for example `timeNow func() time.Time` field.
For production code, you can initialize the field with `time.Now`. For test code, you can set a custom time that will be used by the struct.

<table>
<thead align="center"><tr><th>Avoid 🔥</th></tr></thead>
<tbody>
<tr><td>


```go
func (s *SomeType) IsExpired(created time.Time) bool {
    // Code is hardly testable.
    return time.Since(created) >= s.expiryDuration
}
```

</td></tr>
</tbody></table>
<table>
<thead align="center"><tr><th>Better 🤓</th></tr></thead>
<tbody>
<tr><td>


```go
func (s *SomeType) IsExpired(created time.Time) bool {
    // s.timeNow is time.Now on production, mocked in tests.
    return created.Add(s.expiryDuration).After(s.timeNow())
}
```

</td></tr>
</tbody></table>

## Ensured by linters

This is the list of rules we ensure automatically. This section is for those who are curious why such linting rules
were added or want to maybe add similar to their project.

#### Avoid Prints.

Never use `print`. Always use a passed `go-kit/log.Logger`.

#### Ensure Prometheus Metric Registration

#### go vet

#### golangci-lint

#### misspell

#### Commentaries Should we a Full Sentence.

# Bash

Overall try to NOT use bash. For scripts longer than 30 lines, consider writing it in Go as we did [here](https://github.com/thanos-io/thanos/blob/55cb8ca38b3539381dc6a781e637df15c694e50a/scripts/copyright/copyright.go).

If you have to, we follow the Google Shell style guide: https://google.github.io/styleguide/shellguide.html
