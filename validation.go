package revel

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
)

//  Exsample of useing the Validation.
//  c.Validation.Required(myName).Message("Your name is required")
// 	c.Validation.MinSize(myName, 3).Message("Your name is not long enough")
// 	if c.Validation.HasErrors() {
// 		c.Validation.Keep()
// 		c.FlashParams()
// 		return c.Redirect(Application.Index)
// 	}

type ValidationError struct {
	Message, Key string
}

// Returns the Message.
func (e *ValidationError) String() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// A Validation context manages data validation and error messages.
type Validation struct {
	Errors []*ValidationError
	keep   bool
}

// Tell Revel to serialize the ValidationErrors to the Flash cookie.
// Controller.FlashParams()    -->     Controller.Redirect(Action)
func (v *Validation) Keep() {
	v.keep = true
}

func (v *Validation) Clear() {
	v.Errors = []*ValidationError{}
}

// 如果 validation context 非空的话就返回 true
// 以此来判断是否有 validation error 发生
func (v *Validation) HasErrors() bool {
	return len(v.Errors) > 0
}

// Return the errors mapped by key.
// If there are multiple validation errors associated with a single key, the
// first one "wins".  (Typically the first validation will be the more basic).
func (v *Validation) ErrorMap() map[string]*ValidationError {
	m := map[string]*ValidationError{}
	for _, e := range v.Errors {
		if _, ok := m[e.Key]; !ok {
			m[e.Key] = e
		}
	}
	return m
}

// Add an error to the validation context.
func (v *Validation) Error(message string, args ...interface{}) *ValidationResult {
	return (&ValidationResult{
		Ok:    false,
		Error: &ValidationError{},
	}).Message(message, args)
}

// A ValidationResult is returned from every validation method.
// It provides an indication of success, and a pointer to the Error (if any).
// Each evaluation returns a ValidationResult. Failed ValidationResults are stored in the Validation context.
type ValidationResult struct {
	Error *ValidationError
	Ok    bool
}

func (r *ValidationResult) Key(key string) *ValidationResult {
	if r.Error != nil {
		r.Error.Key = key
	}
	return r
}

func (r *ValidationResult) Message(message string, args ...interface{}) *ValidationResult {
	if r.Error != nil {
		if len(args) == 0 {
			r.Error.Message = message
		} else {
			r.Error.Message = fmt.Sprintf(message, args)
		}
	}
	return r
}

// Test that the argument is non-nil and non-empty (if string or list)
func (v *Validation) Required(obj interface{}) *ValidationResult {
	return v.apply(Required{}, obj)
}

func (v *Validation) Min(n int, min int) *ValidationResult {
	return v.apply(Min{min}, n)
}

func (v *Validation) Max(n int, max int) *ValidationResult {
	return v.apply(Max{max}, n)
}

func (v *Validation) Range(n, min, max int) *ValidationResult {
	return v.apply(Range{Min{min}, Max{max}}, n)
}

func (v *Validation) MinSize(obj interface{}, min int) *ValidationResult {
	return v.apply(MinSize{min}, obj)
}

func (v *Validation) MaxSize(obj interface{}, max int) *ValidationResult {
	return v.apply(MaxSize{max}, obj)
}

func (v *Validation) Length(obj interface{}, n int) *ValidationResult {
	return v.apply(Length{n}, obj)
}

func (v *Validation) Match(str string, regex *regexp.Regexp) *ValidationResult {
	return v.apply(Match{regex}, str)
}

func (v *Validation) Email(str string) *ValidationResult {
	return v.apply(Email{Match{emailPattern}}, str)
}

// As part of building the app, Revel records the name of the variable being validated, 
// and uses that as the default key in the validation context (to be looked up later).
// 我们检测到的错误是使用变量的名字作为key, 保存在 validation context中的，这样我们在后面就很容易的分辨出是那个值的什么类型的无效
// 比如:
//	 func (c Application) Hello(myName string) revel.Result {
// 		c.Validation.Required(myName).Message("Your name is required")
// 		c.Validation.MinSize(myName, 3).Message("Your name is not long enough")
// 		if c.Validation.HasErrors() {
// 			c.Validation.Keep()
// 			c.FlashParams()
// 			return c.Redirect(Application.Index)
// 		}
// 		return c.Render(myName)
// 	}
// 我们会到 errors map 里面去检索 myName 这个键上的 error 信息，这样我们就可以在template 中去使用，比如:
// 		{{range .errors}}
// 			<li> {{.Message}}
// 		{{end}}
// or.
// 	<p class="{{if .errors.myName}}error{{end}}">
// 		<input name="myName" value="{{.flash.myName}}"/>
// 		<span class="error">{{.errors.myName.Message}}</span>
// 	</p>

func (v *Validation) apply(chk Validator, obj interface{}) *ValidationResult {
	if chk.IsSatisfied(obj) {
		return &ValidationResult{Ok: true}
	}

	// Get the default key.
	var key string
	if pc, _, line, ok := runtime.Caller(2); ok {
		f := runtime.FuncForPC(pc)
		if defaultKeys, ok := DefaultValidationKeys[f.Name()]; ok {
			key = defaultKeys[line]
		}
	} else {
		INFO.Println("Failed to get Caller information to look up Validation key")
	}

	// Add the error to the validation context.
	err := &ValidationError{
		Message: chk.DefaultMessage(),
		Key:     key,
	}
	v.Errors = append(v.Errors, err)

	// Also return it in the result.
	return &ValidationResult{
		Ok:    false,
		Error: err,
	}
}

// Apply a group of validators to a field, in order, and return the
// ValidationResult from the first one that fails, or the last one that
// succeeds.
func (v *Validation) Check(obj interface{}, checks ...Validator) *ValidationResult {
	var result *ValidationResult
	for _, check := range checks {
		result = v.apply(check, obj)
		if !result.Ok {
			return result
		}
	}
	return result
}

type ValidationPlugin struct{ EmptyPlugin }

func (p ValidationPlugin) BeforeRequest(c *Controller) {
	c.Validation = &Validation{
		Errors: restoreValidationErrors(c.Request.Request),
		keep:   false,
	}
}

func (p ValidationPlugin) AfterRequest(c *Controller) {
	// Add Validation errors to RenderArgs.
	c.RenderArgs["errors"] = c.Validation.ErrorMap()

	// Store the Validation errors
	var errorsValue string
	if c.Validation.keep {
		for _, error := range c.Validation.Errors {
			if error.Message != "" {
				errorsValue += "\x00" + error.Key + ":" + error.Message + "\x00"
			}
		}
	}
	c.SetCookie(&http.Cookie{
		Name:  CookiePrefix + "_ERRORS",
		Value: url.QueryEscape(errorsValue),
		Path:  "/",
	})
}

// Restore Validation.Errors from a request.
func restoreValidationErrors(req *http.Request) []*ValidationError {
	errors := make([]*ValidationError, 0, 5)
	if cookie, err := req.Cookie(CookiePrefix + "_ERRORS"); err == nil {
		ParseKeyValueCookie(cookie.Value, func(key, val string) {
			errors = append(errors, &ValidationError{
				Key:     key,
				Message: val,
			})
		})
	}
	return errors
}

// Register default validation keys for all calls to Controller.Validation.Func().
// Map from (package).func => (line => name of first arg to Validation func)
// E.g. "myapp/controllers.helper" or "myapp/controllers.(*Application).Action"
// This is set on initialization in the generated main.go file.
var DefaultValidationKeys map[string]map[int]string
