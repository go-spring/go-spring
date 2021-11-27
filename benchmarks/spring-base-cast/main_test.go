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

package main_test

import (
	"testing"
	"time"

	SpringCast "github.com/go-spring/spring-base/cast"
	"github.com/spf13/cast"
)

func BenchmarkToBool(b *testing.B) {

	// bool/go-spring-8  300376836 3.79 ns/op
	// bool/spf13/cast-8 220581253 5.28 ns/op
	b.Run("bool", func(b *testing.B) {
		v := true
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToBoolE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToBoolE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *bool/go-spring-8  350380788 3.44 ns/op
	// *bool/spf13/cast-8  39940608	28.4 ns/op
	b.Run("*bool", func(b *testing.B) {
		v := true
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToBoolE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToBoolE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string/go-spring-8  41272039 28.3 ns/op
	// string/spf13/cast-8 37898732 31.2 ns/op
	b.Run("string", func(b *testing.B) {
		v := "true"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToBoolE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToBoolE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *string/go-spring-8  268056079 4.45 ns/op
	// *string/spf13/cast-8  24224218 49.1 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "true"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToBoolE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToBoolE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func BenchmarkToInt(b *testing.B) {

	// int/go-spring-8  79222540 15.4 ns/op
	// int/spf13/cast-8 60851084 18.8 ns/op
	b.Run("int", func(b *testing.B) {
		v := int64(10)
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToInt64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToInt64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *int/go-spring-8  385364796 3.16 ns/op
	// *int/spf13/cast-8  34533686 33.8 ns/op
	b.Run("*int", func(b *testing.B) {
		v := int64(10)
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToInt64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToInt64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string/go-spring-8  26871295 44.7 ns/op
	// string/spf13/cast-8 27892414 44.1 ns/op
	b.Run("string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToInt64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToInt64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *string/go-spring-8  71975913 16.3 ns/op
	// *string/spf13/cast-8 18520660 62.2 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToInt64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToInt64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func BenchmarkToFloat(b *testing.B) {

	// float/go-spring-8  73689636 15.6 ns/op
	// float/spf13/cast-8 64039783 19.0 ns/op
	b.Run("float", func(b *testing.B) {
		v := float64(10)
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToFloat64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToFloat64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *float/go-spring-8  311454639 3.88 ns/op
	// *float/spf13/cast-8  35120750 33.4 ns/op
	b.Run("*float", func(b *testing.B) {
		v := float64(10)
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToFloat64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToFloat64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string/go-spring-8  22259067 47.3 ns/op
	// string/spf13/cast-8 24166567 51.0 ns/op
	b.Run("string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToFloat64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToFloat64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *string/go-spring-8  49874940 22.2 ns/op
	// *string/spf13/cast-8 16239358 70.0 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToFloat64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToFloat64E(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func BenchmarkToString(b *testing.B) {

	// int/go-spring-8  60869038 18.2 ns/op
	// int/spf13/cast-8 24028012 50.8 ns/op
	b.Run("int", func(b *testing.B) {
		v := 10
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToStringE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToStringE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *int/go-spring-8  212666425 5.67 ns/op
	// *int/spf13/cast-8  14485808 78.9 ns/op
	b.Run("*int", func(b *testing.B) {
		v := 10
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToStringE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToStringE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string/go-spring-8  39322546 29.3 ns/op
	// string/spf13/cast-8 19548669 62.6 ns/op
	b.Run("string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToStringE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToStringE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *string/go-spring-8  346786574 3.46 ns/op
	// *string/spf13/cast-8  12594800 94.3 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToStringE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToStringE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func BenchmarkToDuration(b *testing.B) {

	// int64/go-spring-8  77523229 14.8 ns/op
	// int64/spf13/cast-8 49373287 23.2 ns/op
	b.Run("int64", func(b *testing.B) {
		v := time.Now().UnixNano()
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToDurationE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToDurationE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *int64/go-spring-8  310761321 3.85 ns/op
	// *int64/spf13/cast-8 30532450  38.4 ns/op
	b.Run("*int64", func(b *testing.B) {
		v := time.Now().UnixNano()
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToDurationE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToDurationE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string/go-spring-8  18037459 66.7 ns/op
	// string/spf13/cast-8  4729190 259 ns/op
	b.Run("string", func(b *testing.B) {
		v := SpringCast.ToString(time.Now().UnixNano()) + "ns"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToDurationE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToDurationE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *string/go-spring-8  28039998 40.8 ns/op
	// *string/spf13/cast-8 4369104	 273 ns/op
	b.Run("*string", func(b *testing.B) {
		v := SpringCast.ToString(time.Now().UnixNano()) + "ns"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToDurationE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToDurationE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func BenchmarkToTime(b *testing.B) {

	// int64/go-spring-8  58398465 17.4 ns/op
	// int64/spf13/cast-8 62458393 18.6 ns/op
	b.Run("int64", func(b *testing.B) {
		v := time.Now().Unix()
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToTimeE(v, SpringCast.TimeArg{Unit: time.Second})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToTimeE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// *int64/go-spring-8  220410337 5.61 ns/op
	// *int64/spf13/cast-8 33917301  35.9 ns/op
	b.Run("*int64", func(b *testing.B) {
		v := time.Now().Unix()
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToTimeE(&v, SpringCast.TimeArg{Unit: time.Second})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToTimeE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string/go-spring-8  3559998 324 ns/op
	// string/spf13/cast-8 332461  3490 ns/op
	b.Run("string", func(b *testing.B) {
		format := "2006-01-02 15:04:05 -0700"
		v := time.Now().Format(format)
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToTimeE(v, SpringCast.TimeArg{Format: format})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToTimeE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// string#01/go-spring-8  3701358 279 ns/op
	// string#01/spf13/cast-8 316292  3498 ns/op
	b.Run("string", func(b *testing.B) {
		format := "2006-01-02 15:04:05 -0700"
		v := time.Now().Format(format)
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := SpringCast.ToTimeE(&v, SpringCast.TimeArg{Format: format})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToTimeE(&v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
