<!--
  Licensed to the Apache Software Foundation (ASF) under one or more
  contributor license agreements. See the NOTICE file distributed with
  this work for additional information regarding copyright ownership.
  The ASF licenses this file to You under the Apache License, Version 2.0
  (the "License"); you may not use this file except in compliance with
  the License. You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
-->

# RFC 0000: Input hashing

**Draft.** This proposal defines the bytes that interoperable sketches and filters feed to their hash functions. Each family specifies its supported item types, hash algorithm, seeds, and how hashes select counters, registers, or filter bits. For example, [Count-Min v2](0021-count-min-format.md#4-item-encoding) supports integers, strings, and bytes. Its floating-point input remains undefined.

In [Discussion #20][discussion], Java inserted an empty byte array and `-0.0` into separate Bloom filters. Java found each item. C++ successfully read both images and returned `false` for both items. For empty bytes the C++ query returned early. For negative zero it probed different bits.

## Proposed rules

**Integers.** Encode 64-bit integers as eight little-endian bytes, using two’s complement for signed values. Sign-extend narrower signed integers. For narrower unsigned integers the family must name its widening rule. Count-Min v2 preserves the unsigned value. The [C++ HLL overload][cpp-hll] interprets the high bit as a sign bit at the original width. Both cases appear in the vectors below.

**Strings and bytes.** Encode a sequence of Unicode scalar values as UTF-8. Preserve their order and spelling, including embedded NUL. Hash a byte sequence verbatim. Both forms omit length prefixes, type tags, and terminators. The text `"é"` (U+00E9) and the two bytes `c3 a9` therefore represent the same input. Historical UTF-16 overloads such as Java’s `char[]` have a separate input contract.

**Floating point.** Where the family accepts floating-point items, widen binary32 to binary64. Encode both signed zeros as `0x0000000000000000` and every NaN, regardless of sign or payload, as `0x7ff8000000000000`. For every other value, including infinities, use its binary64 bits. Feed the resulting eight bytes to the hash in little-endian order.

**Null and empty items.** Skip a null item in an API that accepts nulls, an empty string, or a zero-length byte sequence. Updates and filter builds omit that item. Membership queries return `false`. Query-and-update returns `false` and leaves the filter unchanged. Count-Min estimates and both bounds return zero even on a populated sketch. A one-byte input `00` is a regular item. Families define any additional types, including arrays of numbers and composite keys, and their output conventions separately.

**API contract.** Apply the same rules when writing and querying. A typed API can enforce them internally. A generic API can document a wrapper and a caller-side empty check. Conformance tests must exercise the documented calling convention. For example, Rust Bloom callers use `raw_bytes` for strings and bytes and skip empty inputs before calling the filter. [Rust Bloom guidance][rust-bloom]

## Conformance

These vectors give the input bytes in order and the unsigned hexadecimal digest of XXH64 with seed zero. The digest checks the encoding in isolation. Each family still uses its own hash and seeds.

| Input                                    | Bytes (hex)               | XXH64, seed 0      |
|------------------------------------------|---------------------------|--------------------|
| `i8(-1)` or sign-extended `u8(255)`      | `ff ff ff ff ff ff ff ff` | `85d136adb773c6c9` |
| `u8(255)`, preserving its unsigned value | `ff 00 00 00 00 00 00 00` | `2aa06adfe623603f` |
| U+00E9 or raw bytes `c3 a9`              | `c3 a9`                   | `17d757dfb8b46f78` |
| One zero byte                            | `00`                      | `e934a84adb052768` |
| `+0.0` or `-0.0`                         | `00 00 00 00 00 00 00 00` | `34c96acdcadb1bbb` |
| `1.5` as binary32 or binary64            | `00 00 00 00 00 00 f8 3f` | `49f7b96b6b5ccaf9` |
| Any NaN                                  | `00 00 00 00 00 00 f8 7f` | `e9adb09fee122aac` |

For each supported writer/reader pair, insert one sign of zero and query the other after deserialization. Repeat with NaN bit patterns `0x7ff8000000000001` and `0xfff8000000000042`, then swap writer and reader where supported. On empty and populated structures, check the return value and state after each skipped operation. Include a single zero byte as a positive control. For sketches without item queries, compare observable state after equivalent update streams.

## Evidence at pinned revisions

- **Java and C++.** The [family-by-family comparison][discussion] records opposite empty-byte behavior. Java HLL, CPC, Theta, Tuple keys, and Count-Min v1 skip the update. C++ hashes a non-null pointer of length zero in those families. For Bloom it is Java that hashes the empty bytes. Java Bloom and XOR also preserve the sign of zero, while C++ Bloom canonicalizes it. [Java Bloom][java-bloom] · [C++ Bloom][cpp-bloom]
- **Go.** Bloom hashes a nil or empty byte slice on update, query, and query-and-update, skips empty strings, and normalizes scalar signed zero and NaNs. Count-Min v1 skips empty byte updates and returns a zero estimate, but its upper bound adds a term based on total weight. An empty sketch alone hides this difference. [Bloom][go-bloom] · [Count-Min][go-count]
- **Rust.** The generic APIs accept `Hash` values. The documented wrappers supply raw bytes, canonical floats, and two integer widening strategies. HLL documents sign extension and Bloom documents value-preserving widening. The caller handles empty inputs. [Hash wrappers][rust-hash] · [HLL][rust-hll] · [Bloom][rust-bloom]
- **Python.** The Count-Min binding exposes integer and string overloads for update, estimate, and bounds. Its pinned C++ 5.3.0 backend skips empty strings and returns zero for all three queries. The binding exposes no separate raw-buffer overload for these operations. [Binding][py-count] · [Backend pin][py-pin] · [String operations][py-backend]

## Legacy images and open decisions

A Java Bloom image containing only `-0.0` retains the positions chosen by its writer. Updating a reader to normalize zero cannot move those bits. A serialization version (SerVer) can identify a new contract. Older images of the same version may carry different producer conventions. Each affected family must specify which legacy queries and mixed-version operations it can support, when to reject an image, and when users need the original items to rebuild it.

Review is requested on treating all empty byte sequences as absent and on the family-specific unsigned widening rules. Before rollout, complete the inventory by family, overload, and operation across Java, C++, Go, Rust, and Python, recording any caller steps. Use it to choose a legacy policy per affected family and gate cross-language fixtures by the versions each reader supports. [Compatibility discussion][legacy] · [Proposed scope][scope]

[discussion]: https://github.com/apache/datasketches-tck/discussions/20
[legacy]: https://github.com/apache/datasketches-tck/discussions/20#discussioncomment-19530843
[scope]: https://github.com/apache/datasketches-tck/discussions/20#discussioncomment-19585778
[cpp-hll]: https://github.com/apache/datasketches-cpp/blob/c22f2b1d41497ff2d331e74d1a65be3cd17b1dc3/hll/include/HllSketch-internal.hpp#L131-L144
[java-bloom]: https://github.com/apache/datasketches-java/blob/ef440a814319f57d165dfbe2c39fc07bf4a0a1e7/src/main/java/org/apache/datasketches/filters/bloomfilter/BloomFilter.java#L278-L311
[cpp-bloom]: https://github.com/apache/datasketches-cpp/blob/c22f2b1d41497ff2d331e74d1a65be3cd17b1dc3/filters/include/bloom_filter_impl.hpp#L812-L838
[go-bloom]: https://github.com/apache/datasketches-go/blob/c9701e7d4c76952c615c3f2938f7d61bf126b561/filters/bloom_filter.go#L262-L408
[go-count]: https://github.com/apache/datasketches-go/blob/c9701e7d4c76952c615c3f2938f7d61bf126b561/count/count_min_sketch.go#L113-L181
[rust-hash]: https://github.com/apache/datasketches-rust/blob/034de1a69a5989e70d2ac03545d181904e895014/datasketches/src/hash/value/mod.rs#L18-L35
[rust-hll]: https://github.com/apache/datasketches-rust/blob/034de1a69a5989e70d2ac03545d181904e895014/datasketches/src/hll/mod.rs#L25-L34
[rust-bloom]: https://github.com/apache/datasketches-rust/blob/034de1a69a5989e70d2ac03545d181904e895014/datasketches/src/bloom/mod.rs#L29-L39
[py-count]: https://github.com/apache/datasketches-python/blob/bd0231d9512bc62b6ff45efd752dc4f4ae4c57e3/src/count_wrapper.cpp#L69-L84
[py-pin]: https://github.com/apache/datasketches-python/blob/bd0231d9512bc62b6ff45efd752dc4f4ae4c57e3/CMakeLists.txt#L117-L122
[py-backend]: https://github.com/apache/datasketches-cpp/blob/5b5524f55d121225a7909c2851271327684585d7/count/include/count_min_impl.hpp#L149-L221
