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

	SpringCast "github.com/go-spring/spring-base/cast"
	"github.com/spf13/cast"
)

func BenchmarkToBool(b *testing.B) {

	// bool/go-spring-8  167151211  7.4 ns/op
	// bool/spf13/cast-8 100000000 11.4 ns/op
	b.Run("bool", func(b *testing.B) {
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToBoolE(true)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToBoolE(true)
			}
		})
	})

	// *bool/go-spring-8  205132816  5.8 ns/op
	// *bool/spf13/cast-8  16189530 72.9 ns/op
	b.Run("*bool", func(b *testing.B) {
		v := true
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToBoolE(&v)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToBoolE(&v)
			}
		})
	})

	// string/go-spring-8  61645167 19.1 ns/op
	// string/spf13/cast-8 45214893 25.5 ns/op
	b.Run("string", func(b *testing.B) {
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToBoolE("true")
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToBoolE("true")
			}
		})
	})

	// *string/go-spring-8  57009838  19 ns/op
	// *string/spf13/cast-8  9416020 122 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "true"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToBoolE(&v)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToBoolE(&v)
			}
		})
	})
}

func BenchmarkToInt(b *testing.B) {

	// int/go-spring-8  175706412  6.8 ns/op
	// int/spf13/cast-8  91555164 13.1 ns/op
	b.Run("int", func(b *testing.B) {
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToInt64E(10)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToInt64E(10)
			}
		})
	})

	// *int/go-spring-8  203744269  5.9 ns/op
	// *int/spf13/cast-8  12740619 85.6 ns/op
	b.Run("*int", func(b *testing.B) {
		v := 10
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToInt64E(&v)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToInt64E(&v)
			}
		})
	})

	// string/go-spring-8  27537498 43.1 ns/op
	// string/spf13/cast-8 24642079 49.1 ns/op
	b.Run("string", func(b *testing.B) {
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToInt64E("10")
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToInt64E("10")
			}
		})
	})

	// *string/go-spring-8  26194868  43 ns/op
	// *string/spf13/cast-8  7249736 157 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToInt64E(&v)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToInt64E(&v)
			}
		})
	})
}

func BenchmarkToString(b *testing.B) {

	// int/go-spring-8  81809098  13 ns/op
	// int/spf13/cast-8 10077415 122 ns/op
	b.Run("int", func(b *testing.B) {
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToStringE(10)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToStringE(10)
			}
		})
	})

	// *int/go-spring-8  89229592  13 ns/op
	// *int/spf13/cast-8  5505238 212 ns/op
	b.Run("*int", func(b *testing.B) {
		v := 10
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToStringE(&v)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToStringE(&v)
			}
		})
	})

	// string/go-spring-8  184601107   6 ns/op
	// string/spf13/cast-8  11421825 103 ns/op
	b.Run("string", func(b *testing.B) {
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToStringE("10")
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToStringE("10")
			}
		})
	})

	// *string/go-spring-8  184164370   6 ns/op
	// *string/spf13/cast-8   5597317 212 ns/op
	b.Run("*string", func(b *testing.B) {
		v := "10"
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SpringCast.ToStringE(&v)
			}
		})
		b.Run("spf13/cast", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = cast.ToStringE(&v)
			}
		})
	})
}
