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

//SADD
//redis> SADD myset "Hello"
//(integer) 1
//redis> SADD myset "World"
//(integer) 1
//redis> SADD myset "World"
//(integer) 0
//redis> SMEMBERS myset
//1) "World"
//2) "Hello"
//redis>

//SCARD
//redis> SADD myset "Hello"
//(integer) 1
//redis> SADD myset "World"
//(integer) 1
//redis> SCARD myset
//(integer) 2
//redis>

//SDIFF
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SDIFF key1 key2
//1) "b"
//2) "a"
//redis>

//SDIFFSTORE
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SDIFFSTORE key key1 key2
//(integer) 2
//redis> SMEMBERS key
//1) "b"
//2) "a"
//redis>

//SINTER
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SINTER key1 key2
//1) "c"
//redis>

//SINTERSTORE
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SINTERSTORE key key1 key2
//(integer) 1
//redis> SMEMBERS key
//1) "c"
//redis>

//SISMEMBER
//redis> SADD myset "one"
//(integer) 1
//redis> SISMEMBER myset "one"
//(integer) 1
//redis> SISMEMBER myset "two"
//(integer) 0
//redis>

//SMEMBERS
//redis> SADD myset "Hello"
//(integer) 1
//redis> SADD myset "World"
//(integer) 1
//redis> SMEMBERS myset
//1) "World"
//2) "Hello"
//redis>

//SMISMEMBER
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "one"
//(integer) 0
//redis> SMISMEMBER myset "one" "notamember"
//1) (integer) 1
//2) (integer) 0
//redis>

//SMOVE
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "two"
//(integer) 1
//redis> SADD myotherset "three"
//(integer) 1
//redis> SMOVE myset myotherset "two"
//(integer) 1
//redis> SMEMBERS myset
//1) "one"
//redis> SMEMBERS myotherset
//1) "three"
//2) "two"
//redis>

//SPOP
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "two"
//(integer) 1
//redis> SADD myset "three"
//(integer) 1
//redis> SPOP myset
//"one"
//redis> SMEMBERS myset
//1) "three"
//2) "two"
//redis> SADD myset "four"
//(integer) 1
//redis> SADD myset "five"
//(integer) 1
//redis> SPOP myset 3
//1) "three"
//2) "four"
//3) "five"
//redis> SMEMBERS myset
//1) "two"
//redis>

//SRANDMEMBER
//redis> SADD myset one two three
//(integer) 3
//redis> SRANDMEMBER myset
//"two"
//redis> SRANDMEMBER myset 2
//1) "three"
//2) "two"
//redis> SRANDMEMBER myset -5
//1) "two"
//2) "one"
//3) "one"
//4) "three"
//5) "three"
//redis>

//SREM
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "two"
//(integer) 1
//redis> SADD myset "three"
//(integer) 1
//redis> SREM myset "one"
//(integer) 1
//redis> SREM myset "four"
//(integer) 0
//redis> SMEMBERS myset
//1) "three"
//2) "two"
//redis>

//SUNION
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SUNION key1 key2
//1) "b"
//2) "c"
//3) "e"
//4) "a"
//5) "d"
//redis>

//SUNIONSTORE
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SUNIONSTORE key key1 key2
//(integer) 5
//redis> SMEMBERS key
//1) "b"
//2) "c"
//3) "e"
//4) "a"
//5) "d"
//redis>
