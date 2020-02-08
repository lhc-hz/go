/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package SpringCore

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

func init() {

	// string -> time.Duration converter
	// time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"。
	RegisterTypeConverter(func(v string) time.Duration {
		r, err := cast.ToDurationE(v)
		SpringUtils.Panic(err).When(err != nil)
		return r
	})

	// string -> time.Time converter
	// 支持非常多的日期格式，参见 cast.StringToDate。
	RegisterTypeConverter(func(v string) time.Time {
		r, err := cast.ToTimeE(v)
		SpringUtils.Panic(err).When(err != nil)
		return r
	})
}

// defaultProperties Properties 的默认版本
type defaultProperties struct {
	properties map[string]interface{}
}

// NewDefaultProperties defaultProperties 的构造函数
func NewDefaultProperties() *defaultProperties {
	return &defaultProperties{
		properties: make(map[string]interface{}),
	}
}

func (p *defaultProperties) readProperties(r func(*viper.Viper) error) {

	v := viper.New()
	if err := r(v); err != nil {
		panic(err)
	}

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		p.SetProperty(key, val)
		SpringLogger.Tracef("%s=%v", key, val)
	}
}

// LoadProperties 加载属性配置文件，支持 properties、yaml 和 toml 三种文件格式。
func (p *defaultProperties) LoadProperties(filename string) {
	SpringLogger.Debug(">>> load properties from file: ", filename)

	p.readProperties(func(v *viper.Viper) error {
		v.SetConfigFile(filename)
		return v.ReadInConfig()
	})
}

// ReadProperties 读取属性配置文件，支持 properties、yaml 和 toml 三种文件格式。
func (p *defaultProperties) ReadProperties(reader io.Reader, configType string) {
	SpringLogger.Debug(">>> load properties from reader type: ", configType)

	p.readProperties(func(v *viper.Viper) error {
		v.SetConfigType(configType)
		return v.ReadConfig(reader)
	})
}

// GetProperty 返回属性值，属性名称统一转成小写。
func (p *defaultProperties) GetProperty(key string) interface{} {
	return p.properties[strings.ToLower(key)]
}

// GetBoolProperty 返回布尔型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetBoolProperty(key string) bool {
	return cast.ToBool(p.GetProperty(key))
}

// GetIntProperty 返回有符号整型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetIntProperty(key string) int64 {
	return cast.ToInt64(p.GetProperty(key))
}

// GetUintProperty 返回无符号整型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetUintProperty(key string) uint64 {
	return cast.ToUint64(p.GetProperty(key))
}

// GetFloatProperty 返回浮点型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetFloatProperty(key string) float64 {
	return cast.ToFloat64(p.GetProperty(key))
}

// GetStringProperty 返回字符串型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetStringProperty(key string) string {
	return cast.ToString(p.GetProperty(key))
}

// GetDurationProperty 返回 Duration 类型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetDurationProperty(key string) time.Duration {
	return cast.ToDuration(p.GetProperty(key))
}

// GetTimeProperty 返回 Time 类型的属性值，属性名称统一转成小写。
func (p *defaultProperties) GetTimeProperty(key string) time.Time {
	return cast.ToTime(p.GetProperty(key))
}

// SetProperty 设置属性值，属性名称统一转成小写。
func (p *defaultProperties) SetProperty(key string, value interface{}) {
	p.properties[strings.ToLower(key)] = value
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值
func (p *defaultProperties) GetDefaultProperty(key string, defaultValue interface{}) (interface{}, bool) {
	if v, ok := p.properties[strings.ToLower(key)]; ok {
		return v, true
	}
	return defaultValue, false
}

// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *defaultProperties) GetPrefixProperties(prefix string) map[string]interface{} {
	prefix = strings.ToLower(prefix)
	result := make(map[string]interface{})
	for k, v := range p.properties {
		if k == prefix || strings.HasPrefix(k, prefix+".") {
			result[k] = v
		}
	}
	return result
}

// GetProperties 返回所有的属性值，属性名称统一转成小写。
func (p *defaultProperties) GetProperties() map[string]interface{} {
	return p.properties
}

// bindOption 属性值绑定可选项
type bindOption struct {
	propNamePrefix string // 属性名前缀
	fullPropName   string // 完整属性名
	fieldName      string // 结构体字段的名称
	allAccess      bool   // 私有字段是否绑定
}

// bindStruct 对结构体进行属性值绑定
func bindStruct(p Properties, v reflect.Value, opt bindOption) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)

		// 可能会开放私有字段
		fv = SpringUtils.ValuePatchIf(fv, opt.allAccess)
		subFieldName := opt.fieldName + ".$" + ft.Name

		// 字段的绑定可选项
		subOpt := bindOption{
			propNamePrefix: opt.propNamePrefix,
			fullPropName:   opt.fullPropName,
			fieldName:      subFieldName,
			allAccess:      opt.allAccess,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			bindStructField(p, fv, tag, subOpt)
			continue
		}

		// 匿名嵌套需要处理，但是不是结构体的具名字段无需处理
		if ft.Anonymous || ft.Type.Kind() == reflect.Struct {
			bindStruct(p, fv, subOpt)
		}
	}
}

// bindStructField 对结构体的字段进行属性绑定
func bindStructField(p Properties, v reflect.Value, tag string, opt bindOption) {

	// 检查 tag 语法是否正确
	if !(strings.HasPrefix(tag, "${") && strings.HasSuffix(tag, "}")) {
		panic(fmt.Errorf("%s 属性绑定的语法发生错误", opt.fieldName))
	}

	// 指针不能作为属性绑定的目标
	if v.Kind() == reflect.Ptr {
		panic(fmt.Errorf("%s 属性绑定的目标不能是指针", opt.fieldName))
	}

	ss := strings.Split(tag[2:len(tag)-1], ":=")

	var (
		propName     string
		defaultValue interface{}
	)

	propName = ss[0]

	// 此处使用最短属性名
	if opt.fullPropName == "" {
		opt.fullPropName = propName
	} else if propName != "" {
		opt.fullPropName = opt.fullPropName + "." + propName
	}

	// 属性名如果有前缀要加上前缀
	if opt.propNamePrefix != "" {
		propName = opt.propNamePrefix + "." + propName
	}

	if len(ss) > 1 {
		defaultValue = ss[1] // 此处无需转换成具体类型
	}

	bindValue(p, v, propName, defaultValue, opt)
}

// bindValue 对任意 value 进行属性绑定
func bindValue(p Properties, v reflect.Value, propName string,
	defaultValue interface{}, opt bindOption) {

	getPropValue := func() interface{} { // 获取最终决定的属性值
		if val, ok := p.GetDefaultProperty(propName, nil); ok {
			return val
		} else {
			if defaultValue != nil {
				return defaultValue
			}

			// 尝试找一下具有相同前缀的属性值的列表
			if prefixValue := p.GetPrefixProperties(propName); len(prefixValue) > 0 {
				return prefixValue
			}

			panic(fmt.Errorf("%s properties \"%s\" not config", opt.fieldName, opt.fullPropName))
		}
	}

	t := v.Type()
	k := t.Kind()

	// 存在值类型转换器的情况下结构体优先使用属性值绑定
	if fn, ok := typeConverters[t]; ok {
		propValue := getPropValue()
		fnValue := reflect.ValueOf(fn)
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(propValue)})
		v.Set(out[0])
		return
	}

	if k == reflect.Struct {
		if defaultValue != nil { // 前面已经校验过是否存在值类型转换器
			panic(fmt.Errorf("%s 结构体字段不能指定默认值", opt.fieldName))
		}

		bindStruct(p, v, bindOption{
			propNamePrefix: propName,
			fullPropName:   opt.fullPropName,
			fieldName:      opt.fieldName,
			allAccess:      opt.allAccess,
		})
		return
	}

	// 获取属性值
	propValue := getPropValue()

	switch k {
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		u := cast.ToUint64(propValue)
		v.SetUint(u)
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		i := cast.ToInt64(propValue)
		v.SetInt(i)
	case reflect.Float64, reflect.Float32:
		f := cast.ToFloat64(propValue)
		v.SetFloat(f)
	case reflect.String:
		s := cast.ToString(propValue)
		v.SetString(s)
	case reflect.Bool:
		b := cast.ToBool(propValue)
		v.SetBool(b)
	case reflect.Slice:
		{
			elemType := v.Type().Elem()
			elemKind := elemType.Kind()

			switch elemKind {
			case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
				panic(errors.New("暂未支持"))
			case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
				i := cast.ToIntSlice(propValue)
				v.Set(reflect.ValueOf(i))
			case reflect.Float64, reflect.Float32:
				panic(errors.New("暂未支持"))
			case reflect.String:
				i := cast.ToStringSlice(propValue)
				v.Set(reflect.ValueOf(i))
			case reflect.Bool:
				b := cast.ToBoolSlice(propValue)
				v.Set(reflect.ValueOf(b))
			default:
				if fn, ok := typeConverters[elemType]; ok { // 首先处理使用类型转换器的场景
					fnValue := reflect.ValueOf(fn)
					s0 := cast.ToStringSlice(propValue)
					sv := reflect.MakeSlice(t, len(s0), len(s0))
					for i, iv := range s0 {
						res := fnValue.Call([]reflect.Value{reflect.ValueOf(iv)})
						sv.Index(i).Set(res[0])
					}
					v.Set(sv)

				} else { // 然后处理结构体字段的场景
					if s, isArray := propValue.([]interface{}); isArray {
						result := reflect.MakeSlice(t, len(s), len(s))
						for i, si := range s {
							if sv, err := cast.ToStringMapE(si); err == nil {
								ev := reflect.New(elemType)
								subPropName := fmt.Sprintf("%s[%d]", propName, i)
								bindStruct(&defaultProperties{sv}, ev.Elem(),
									bindOption{
										fieldName:    opt.fieldName,
										fullPropName: subPropName,
										allAccess:    opt.allAccess,
									})
								result.Index(i).Set(ev.Elem())
							} else {
								panic(fmt.Errorf("property %s isn't []map[string]interface{}", opt.fullPropName))
							}
						}
						v.Set(result)
					} else {
						panic(fmt.Errorf("property %s isn't []map[string]interface{}", opt.fullPropName))
					}
				}
			}
		}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			panic(fmt.Errorf("field: %s isn't map[string]interface{}", opt.fieldName))
		}

		elemType := t.Elem()
		elemKind := elemType.Kind()

		switch elemKind {
		case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
			panic(errors.New("暂未支持"))
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
			panic(errors.New("暂未支持"))
		case reflect.Float64, reflect.Float32:
			panic(errors.New("暂未支持"))
		case reflect.Bool:
			panic(errors.New("暂未支持"))
		case reflect.String:
			if mapValue, err := cast.ToStringMapStringE(propValue); err == nil {
				prefix := propName + "."
				result := make(map[string]string)
				for k0, v0 := range mapValue {
					k0 = strings.TrimPrefix(k0, prefix)
					result[k0] = v0
				}
				v.Set(reflect.ValueOf(result))
			} else {
				panic(fmt.Errorf("property %s isn't map[string]string", opt.fullPropName))
			}
		default:
			if fn, ok := typeConverters[elemType]; ok { // 首先处理使用类型转换器的场景
				if mapValue, err := cast.ToStringMapStringE(propValue); err == nil {
					prefix := propName + "."
					fnValue := reflect.ValueOf(fn)
					result := reflect.MakeMap(t)
					for k0, v0 := range mapValue {
						res := fnValue.Call([]reflect.Value{reflect.ValueOf(v0)})
						k0 = strings.TrimPrefix(k0, prefix)
						result.SetMapIndex(reflect.ValueOf(k0), res[0])
					}
					v.Set(result)
				} else {
					panic(fmt.Errorf("property %s isn't map[string]string", opt.fullPropName))
				}

			} else { // 然后处理结构体字段的场景
				if mapValue, err := cast.ToStringMapE(propValue); err == nil {
					temp := make(map[string]map[string]interface{})
					trimKey := propName + "."

					for k0, v0 := range mapValue {
						k0 = strings.TrimPrefix(k0, trimKey)
						sk := strings.Split(k0, ".")
						var item map[string]interface{}
						if item, ok = temp[sk[0]]; !ok {
							item = make(map[string]interface{})
							temp[sk[0]] = item
						}
						item[sk[1]] = v0
					}

					result := reflect.MakeMapWithSize(t, len(temp))
					for k1, v1 := range temp {
						ev := reflect.New(elemType)
						subPropName := fmt.Sprintf("%s.%s", propName, k1)
						bindStruct(&defaultProperties{v1}, ev.Elem(),
							bindOption{
								fieldName:    opt.fieldName,
								fullPropName: subPropName,
								allAccess:    opt.allAccess,
							})
						result.SetMapIndex(reflect.ValueOf(k1), ev.Elem())
					}

					v.Set(result)
				} else {
					panic(fmt.Errorf("property %s isn't map[string]map[string]interface{}", opt.fullPropName))
				}
			}
		}
	default:
		panic(errors.New(opt.fieldName + " unsupported type " + v.Kind().String()))
	}
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func (p *defaultProperties) BindProperty(key string, i interface{}) {
	p.BindPropertyIf(key, i, false)
}

// BindPropertyIf 根据类型获取属性值，属性名称统一转成小写。
func (p *defaultProperties) BindPropertyIf(key string, i interface{}, allAccess bool) {
	v := reflect.ValueOf(i)

	if v.Kind() != reflect.Ptr {
		panic(errors.New("参数 v 必须是一个指针"))
	}

	t := v.Type().Elem()

	s := t.Name() // 当绑定对象是 map 或者 slice 时，取元素的类型名
	if s == "" && (t.Kind() == reflect.Map || t.Kind() == reflect.Slice) {
		s = t.Elem().Name()
	}

	bindValue(p, v.Elem(), key, nil, bindOption{
		fieldName:    s,
		fullPropName: key,
		allAccess:    allAccess,
	})
}
