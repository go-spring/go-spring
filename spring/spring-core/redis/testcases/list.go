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

package testcases

//LINDEX
//redis> LPUSH mylist "World"
//(integer) 1
//redis> LPUSH mylist "Hello"
//(integer) 2
//redis> LINDEX mylist 0
//"Hello"
//redis> LINDEX mylist -1
//"World"
//redis> LINDEX mylist 3
//(nil)
//redis>

//LINSERT
//redis> RPUSH mylist "Hello"
//(integer) 1
//redis> RPUSH mylist "World"
//(integer) 2
//redis> LINSERT mylist BEFORE "World" "There"
//(integer) 3
//redis> LRANGE mylist 0 -1
//1) "Hello"
//2) "There"
//3) "World"
//redis>

//LLEN
//redis> LPUSH mylist "World"
//(integer) 1
//redis> LPUSH mylist "Hello"
//(integer) 2
//redis> LLEN mylist
//(integer) 2
//redis>

//LMOVE
//redis> RPUSH mylist "one"
//(integer) 1
//redis> RPUSH mylist "two"
//(integer) 2
//redis> RPUSH mylist "three"
//(integer) 3
//redis> LMOVE mylist myotherlist RIGHT LEFT
//"three"
//redis> LMOVE mylist myotherlist LEFT RIGHT
//"one"
//redis> LRANGE mylist 0 -1
//1) "two"
//redis> LRANGE myotherlist 0 -1
//1) "three"
//2) "one"
//redis>

//LMPOP
//redis> LMPOP 2 non1 non2 LEFT COUNT 10
//ERR Don't know what to do for "lmpop"
//redis> LPUSH mylist "one" "two" "three" "four" "five"
//(integer) 5
//redis> LMPOP 1 mylist LEFT
//ERR Don't know what to do for "lmpop"
//redis> LRANGE mylist 0 -1
//1) "five"
//2) "four"
//3) "three"
//4) "two"
//5) "one"
//redis> LMPOP 1 mylist RIGHT COUNT 10
//ERR Don't know what to do for "lmpop"
//redis> LPUSH mylist "one" "two" "three" "four" "five"
//(integer) 10
//redis> LPUSH mylist2 "a" "b" "c" "d" "e"
//(integer) 5
//redis> LMPOP 2 mylist mylist2 right count 3
//ERR Don't know what to do for "lmpop"
//redis> LRANGE mylist 0 -1
//1) "five"
// 2) "four"
// 3) "three"
// 4) "two"
// 5) "one"
// 6) "five"
// 7) "four"
// 8) "three"
// 9) "two"
//10) "one"
//redis> LMPOP 2 mylist mylist2 right count 5
//ERR Don't know what to do for "lmpop"
//redis> LMPOP 2 mylist mylist2 right count 10
//ERR Don't know what to do for "lmpop"
//redis> EXISTS mylist mylist2
//(integer) 2
//redis>

//LPOP
//redis> RPUSH mylist "one" "two" "three" "four" "five"
//(integer) 5
//redis> LPOP mylist
//"one"
//redis> LPOP mylist 2
//1) "two"
//2) "three"
//redis> LRANGE mylist 0 -1
//1) "four"
//2) "five"
//redis>

//LPOS
//redis> RPUSH mylist a b c d 1 2 3 4 3 3 3
//(integer) 11
//redis> LPOS mylist 3
//(integer) 6
//redis> LPOS mylist 3 COUNT 0 RANK 2
//1) (integer) 8
//2) (integer) 9
//3) (integer) 10
//redis>

//LPUSH
//redis> LPUSH mylist "world"
//(integer) 1
//redis> LPUSH mylist "hello"
//(integer) 2
//redis> LRANGE mylist 0 -1
//1) "hello"
//2) "world"
//redis>

//LPUSHX
//redis> LPUSH mylist "World"
//(integer) 1
//redis> LPUSHX mylist "Hello"
//(integer) 2
//redis> LPUSHX myotherlist "Hello"
//(integer) 0
//redis> LRANGE mylist 0 -1
//1) "Hello"
//2) "World"
//redis> LRANGE myotherlist 0 -1
//(empty list or set)
//redis>

//LRANGE
//redis> RPUSH mylist "one"
//(integer) 1
//redis> RPUSH mylist "two"
//(integer) 2
//redis> RPUSH mylist "three"
//(integer) 3
//redis> LRANGE mylist 0 0
//1) "one"
//redis> LRANGE mylist -3 2
//1) "one"
//2) "two"
//3) "three"
//redis> LRANGE mylist -100 100
//1) "one"
//2) "two"
//3) "three"
//redis> LRANGE mylist 5 10
//(empty list or set)
//redis>

//LREM
//(integer) 1
//redis> RPUSH mylist "hello"
//(integer) 2
//redis> RPUSH mylist "foo"
//(integer) 3
//redis> RPUSH mylist "hello"
//(integer) 4
//redis> LREM mylist -2 "hello"
//(integer) 2
//redis> LRANGE mylist 0 -1
//1) "hello"
//2) "foo"
//redis>

//LSET
//redis> RPUSH mylist "one"
//(integer) 1
//redis> RPUSH mylist "two"
//(integer) 2
//redis> RPUSH mylist "three"
//(integer) 3
//redis> LSET mylist 0 "four"
//"OK"
//redis> LSET mylist -2 "five"
//"OK"
//redis> LRANGE mylist 0 -1
//1) "four"
//2) "five"
//3) "three"
//redis>

//LTRIM
//redis> RPUSH mylist "one"
//(integer) 1
//redis> RPUSH mylist "two"
//(integer) 2
//redis> RPUSH mylist "three"
//(integer) 3
//redis> LTRIM mylist 1 -1
//"OK"
//redis> LRANGE mylist 0 -1
//1) "two"
//2) "three"
//redis>

//RPOP
//redis> RPUSH mylist "one" "two" "three" "four" "five"
//(integer) 5
//redis> RPOP mylist
//"five"
//redis> RPOP mylist 2
//1) "four"
//2) "three"
//redis> LRANGE mylist 0 -1
//1) "one"
//2) "two"
//redis>

//RPOPLPUSH
//redis> RPUSH mylist "one"
//(integer) 1
//redis> RPUSH mylist "two"
//(integer) 2
//redis> RPUSH mylist "three"
//(integer) 3
//redis> RPOPLPUSH mylist myotherlist
//"three"
//redis> LRANGE mylist 0 -1
//1) "one"
//2) "two"
//redis> LRANGE myotherlist 0 -1
//1) "three"
//redis>

//RPUSH
//redis> RPUSH mylist "hello"
//(integer) 1
//redis> RPUSH mylist "world"
//(integer) 2
//redis> LRANGE mylist 0 -1
//1) "hello"
//2) "world"
//redis>

//RPUSHX
//redis> RPUSH mylist "Hello"
//(integer) 1
//redis> RPUSHX mylist "World"
//(integer) 2
//redis> RPUSHX myotherlist "World"
//(integer) 0
//redis> LRANGE mylist 0 -1
//1) "Hello"
//2) "World"
//redis> LRANGE myotherlist 0 -1
//(empty list or set)
//redis>
