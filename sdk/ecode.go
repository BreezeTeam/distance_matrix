// Package sdk
// @Author Euraxluo  17:05:00
package sdk

import (
	"fmt"
	"github.com/pkg/errors"
)

var (
	//  全局异常管理
	codes = map[int]struct{}{}
)

var (
	Ok              = Define(200, "OK")
	ErrRequest      = Define(400, "请求参数错误")
	ErrNotFind      = Define(404, "没有找到")
	ErrForbidden    = Define(403, "请求被拒绝")
	ErrNoPermission = Define(405, "无权限")
	ErrServer       = Define(500, "服务器错误")
)

// Define only inner error
func Define(code int, msg string) Error {
	if _, ok := codes[code]; ok {
		panic(fmt.Sprintf("ecode: %d already exist", code))
	}
	codes[code] = struct{}{}
	return Error{
		code: code, message: msg,
	}
}

type Errors interface {
	// Error return Code in string form
	Error() string
	// Code get error code.
	Code() int
	// Message get code message.
	Message() string
	// Details get error detail,it may be nil.
	Details() []interface{}
	// Equal for compatible.
	Equal(error) bool
	// Reload Message
	Reload(string) Error
}

type Error struct {
	code    int
	message string
}

func (e Error) Error() string {
	return e.message
}

func (e Error) Message() string {
	return e.message
}

// Reload  如果传入的msg是 Ok 时，不能进行reload
func (e Error) Reload(message string) Error {
	if message != Ok.Message() {
		e.message = message
	}
	return e
}

func (e Error) Code() int {
	return e.code
}

func (e Error) Details() []interface{} { return nil }

func (e Error) Equal(err error) bool { return Equal(err, e) }

// Obtain  字符串转Error，主要用于将可能产生的未定义的异常包装
func Obtain(e string) Error {
	if e == "" {
		return Ok
	}
	return Error{
		code: 500, message: e,
	}
}

// Cause  该函数通过error创建Error
func Cause(err error) Errors {
	if err == nil {
		return Ok
	}
	if ec, ok := errors.Cause(err).(Errors); ok {
		return ec
	}
	return Obtain(err.Error())
}

// Equal 主要用于判断某个异常是否是同类型异常
func Equal(err error, e Error) bool {
	return Cause(err).Code() == e.Code()
}
