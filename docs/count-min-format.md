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

# Count-Min sketch: serialization format, version 2

**Status:** adopted. The design was agreed by lazy consensus in [Discussion #18](https://github.com/apache/datasketches-tck/discussions/18), which closed on 2026-10-25. This spec was posted there on 2026-11-02, and no objections were raised by the 2026-11-09 deadline.

This spec defines everything a Count-Min implementation needs in order to read and write images that work in every language: the byte layout, how row seeds are derived, how items are encoded and hashed, and how a hash maps to a bucket. An image written by any conforming implementation gives the same estimates, updates and merges in every other one. Identical inputs produce byte-identical images.

Version 2 differs from version 1 only in the serial version byte and in the hashing (sections 3 to 5). The byte layout is unchanged.

## 1. Parameters

| Parameter | Type | Valid range |
|---|---|---|
| `numHashes` (rows) | unsigned 8-bit | 1 to 127 (Java and Go store it as a signed byte) |
| `numBuckets` (columns) | unsigned 32-bit | at least 3, and `numHashes × numBuckets < 2^30` |
| `seed` (base seed) | 64-bit | any; the library default is 9001 |

## 2. Byte layout

All multi-byte fields are little-endian.

| Byte offset | Size | Field | Value |
|---:|---:|---|---|
| 0 | 1 | preamble longs | 2 |
| 1 | 1 | serial version | **2** |
| 2 | 1 | family ID | 18 (Count-Min) |
| 3 | 1 | flags | bit 0: `IS_EMPTY`; other bits 0 |
| 4 | 4 | unused | 0 |
| 8 | 4 | `numBuckets` | unsigned 32-bit |
| 12 | 1 | `numHashes` | unsigned 8-bit |
| 13 | 2 | seed hash | the standard DataSketches 16-bit seed hash of `seed`, as used by Theta; 9001 gives `0x93CC` |
| 15 | 1 | unused | 0 |
| 16 | 8 | total weight | signed 64-bit; present only if not empty |
| 24 | 8 × `numHashes` × `numBuckets` | counters | signed 64-bit each; present only if not empty |

- A sketch is empty when its total weight is 0. An empty image is exactly 16 bytes and has `IS_EMPTY` set.
- A non-empty image is `24 + 8 × numHashes × numBuckets` bytes.
- Counters are stored row by row. The counter for row `i`, bucket `b` is at index `i × numBuckets + b`, which is byte offset `24 + 8 × (i × numBuckets + b)`.
- Counters and total weight are signed 64-bit, two's complement. An implementation with a different counter type (such as C++ with a non-`int64_t` weight type) can't write images that other languages read.

Readers check the preamble longs, the family ID, the serial version (section 6) and the seed hash, and that the image is long enough for its declared size.

## 3. Row seeds

Each row hashes items with its own 64-bit row seed, derived from the base seed:

```
rowSeed[i] = XXH64(LE64(i), seed = baseSeed)        for i = 0 .. numHashes - 1
```

- `XXH64` is the 64-bit xxHash, as defined by the [xxHash specification](https://github.com/Cyan4973/xxHash/blob/dev/doc/xxhash_spec.md).
- `LE64(i)` is the row index as 8 bytes, little-endian.
- Nothing is added to the result; in particular there is no `+ baseSeed`.
- Row seeds are never stored in the image. Readers derive them from the seed they are given, after checking the seed hash.

## 4. Item encoding

An item is hashed as a byte sequence. Every implementation must produce exactly the bytes defined in this section. Implementations must not pass items through a language's generic hashing framework, because those add bytes of their own. For example, Rust's `Hash` trait adds `0xFF` after a string and an 8-byte length before a byte slice.

Sections 4.4 to 4.6 are placeholders for input rules that apply to every DataSketches family. They are being discussed in [Discussion #20](https://github.com/apache/datasketches-tck/discussions/20). Each placeholder states what version 2 requires today. When the shared rules are agreed, they will be specified in one place and referenced from here.

### 4.1 Integers

| Item type | Bytes hashed |
|---|---|
| 64-bit integer, signed or unsigned | 8 bytes, little-endian, two's complement |
| Narrower integer | widened to 64 bits first (sign-extended if signed), then as above |

### 4.2 Strings

A string is hashed as its UTF-8 bytes, with no length prefix and no terminator.

- UTF-8 is the only string encoding version 2 defines. An implementation whose native strings use another encoding converts them to UTF-8 before hashing.
- Java has historical APIs in other families that use UTF-16 (for example `update(char[])` in Theta, HLL, CPC, Bloom and Xor, and `ArrayOfUtf16StringsSerDe`). They don't apply to Count-Min.

### 4.3 Byte arrays

A byte array is hashed as the bytes given, in order.

### 4.4 Null and empty items (placeholder)

**Version 2 rule:** an empty string, an empty byte array (zero length, such as Java `new byte[0]`), and a null, in languages that have one, are all ignored. An update does nothing, and a query returns 0.

**Pending (Discussion #20):** a shared policy for every family, covering:
- how a reader or an update interprets a null item;
- how it interprets an empty item (empty string, zero-length byte array, empty array);
- when an implementation may emit, store or return a null or an empty object.

### 4.5 Floating-point items (placeholder)

**Version 2 rule:** floating-point items are not defined for this sketch. Users convert them to one of the types above.

**Pending (Discussion #20):** if a later version accepts floating-point items, they must be canonicalized before hashing, as HLL, CPC, Theta and Tuple already do:
- `-0.0` and `+0.0` must hash as one item, `+0.0`;
- every NaN must hash as one canonical NaN. Java's `Double.doubleToLongBits` produces `0x7FF8000000000000`; other languages must state how they produce the same bits.

### 4.6 Other types (placeholder)

**Version 2 rule:** other types (booleans, arrays of numbers, composite values) are not defined for this sketch. Users convert them to one of the types above.

**Pending:** any later type must define its exact bytes here, independent of any language's in-memory representation.

## 5. Item hashing and bucket mapping

For row `i`:

```
h      = XXH64(itemBytes, seed = rowSeed[i])        // unsigned 64-bit
bucket = (h >>> 1) mod numBuckets
index  = i × numBuckets + bucket
```

The shift by one clears the top bit, so the result is the same whether a language's 64-bit integers are signed or unsigned. Theta, Tuple and the Bloom filter already use this convention.

## 6. Operations and versions

- **Update** `(item, weight)`: the total weight increases by `|weight|`, and in each row the counter at the item's index increases by `weight`.
- **Estimate**: the minimum of the item's counters over all rows. The upper and lower bounds are unchanged from version 1.
- **Merge**: requires equal `numHashes`, `numBuckets` and seed, and the same serial version. Counters and total weights are added.
- **Writing**: new sketches are written as version 2.
- **Reading version 1**: readers accept version 1 images and use the language's own legacy (version 1) row seeds and bucket mapping for them. A sketch read from a version 1 image keeps the legacy hashing and is written back as version 1, so its counters are never labeled with the wrong algorithm. Version 1 sketches can't be merged with version 2 sketches. Version 1 images remain unreliable across languages, and in C++ across standard libraries.
- **Other versions** are rejected.

## 7. Test vectors

These were computed with the DataSketches Java `XxHash64`. The bucket results for seeds 9001 and 42 match the independently computed vectors in [apache/datasketches-rust#299](https://github.com/apache/datasketches-rust/pull/299).

### 7.1 XXH64

| Input | Seed | XXH64 |
|---|---:|---|
| empty | 0 | `0xEF46DB3751D8E999` |
| `"a"` | 0 | `0xD24EC4F1A98C6E5B` |
| `"abc"` | 0 | `0x44BC2CF5AD770999` |

### 7.2 Row seeds

| Base seed | Row 0 | Row 1 | Row 2 |
|---:|---|---|---|
| 9001 | `0xD71D64972584A288` | `0xEFE8454A6499F311` | `0x23819BE21B1B1D9E` |
| 42 | `0xB71B47EBDA15746C` | `0x9ED50FD59358D232` | `0xD19FE30002B9A3CD` |

### 7.3 Item hashes and buckets

Base seed 9001, `numBuckets` = 1000. Each cell shows `XXH64(item, rowSeed[i])` and the bucket.

| Item | Row 0 | Row 1 | Row 2 |
|---|---|---|---|
| integer 1 | `0x9578D22909AB6393` → 185 | `0x73BE69FFC52746B5` → 898 | `0xA16D5BA2EA9D92A9` → 4 |
| integer 9 | `0xE351D001EE0C0725` → 490 | `0x74AB8E217F6C0193` → 217 | `0x4E02535F97CE07D1` → 440 |
| integer -1 | `0x295F6FF9D5F0A170` → 256 | `0x693A5A4C04C19D72` → 641 | `0xC3095F604CF42A69` → 308 |
| integer `0x8000000000000000` | `0x69DD53CB2E1960EE` → 951 | `0xBE1283E430FCA711` → 0 | `0x62BD4CDD3889A7D5` → 298 |
| string `"apple"` | `0x0F686B647380AF74` → 130 | `0x25329624EF6A5FF6` → 747 | `0x501EE67250FD0E21` → 144 |
| string `"été"` (UTF-8 `c3 a9 74 c3 a9`) | `0x38F345607F8C14EC` → 806 | `0x75787F10A22C78A7` → 491 | `0x43BEC0D845165366` → 163 |

Several of these hashes have the top bit set, so an implementation that skips the shift or treats the hash as signed gets different buckets.

With base seed 42, integer 1 maps to buckets 468, 459, 696 and integer 9 to 841, 428, 322.

### 7.4 Images

Base seed 9001, `numHashes` = 2, `numBuckets` = 9.

Empty (16 bytes):

```
02 02 12 01 00 00 00 00  09 00 00 00 02 cc 93 00
```

After `update(1, 10)`, `update(9, 810)` and `update("apple", 5)` (168 bytes). Row 0 puts 9, `"apple"` and 1 in buckets 0, 4 and 6; row 1 puts `"apple"`, 9 and 1 in buckets 1, 2 and 7. The total weight is 825.

```
  0: 02 02 12 00 00 00 00 00   preamble: version 2, family 18, not empty
  8: 09 00 00 00 02 cc 93 00   numBuckets 9, numHashes 2, seed hash 0x93CC
 16: 39 03 00 00 00 00 00 00   total weight 825
 24: 2a 03 00 00 00 00 00 00   row 0, bucket 0: 810
 32: 00 00 00 00 00 00 00 00
 40: 00 00 00 00 00 00 00 00
 48: 00 00 00 00 00 00 00 00
 56: 05 00 00 00 00 00 00 00   row 0, bucket 4: 5
 64: 00 00 00 00 00 00 00 00
 72: 0a 00 00 00 00 00 00 00   row 0, bucket 6: 10
 80: 00 00 00 00 00 00 00 00
 88: 00 00 00 00 00 00 00 00
 96: 00 00 00 00 00 00 00 00   row 1, bucket 0
104: 05 00 00 00 00 00 00 00   row 1, bucket 1: 5
112: 2a 03 00 00 00 00 00 00   row 1, bucket 2: 810
120: 00 00 00 00 00 00 00 00
128: 00 00 00 00 00 00 00 00
136: 00 00 00 00 00 00 00 00
144: 00 00 00 00 00 00 00 00
152: 0a 00 00 00 00 00 00 00   row 1, bucket 7: 10
160: 00 00 00 00 00 00 00 00
```

Reading this image back with seed 9001 gives estimates of 10 for 1, 810 for 9 and 5 for `"apple"`.

## 8. Cross-language test files

Each implementation generates the files below into its `serialization_test_data/<language>_generated_files` directory, as described in [Serialization snapshots](serialization-snapshots.md), and checks the files generated by the other languages.

### 8.1 Names and contents

File names have the form `count_min_v2_<case>_<language>.sk`, where `<language>` is the implementation's snapshot suffix (for example `cpp`, `go`, `java` or `rust`).

Every file uses `numHashes` = 3 and `numBuckets` = 1000.

| Case | Seed | `N` |
|---|---:|---:|
| `n0` | 9001 | 0 (empty) |
| `n1` | 9001 | 1 |
| `n10` | 9001 | 10 |
| `n100` | 9001 | 100 |
| `n1000` | 9001 | 1000 |
| `n10000` | 9001 | 10000 |
| `seed42_n1000` | 42 | 1000 |

For each `i` from 0 to `N - 1`, the sketch is updated twice:
- with the 64-bit integer `i` and weight `i + 1`;
- with the string of `i` in decimal (`"0"`, `"1"`, ..., with no sign, padding or separators) and weight 1.

The total weight is therefore `N × (N + 1) / 2 + N`.

Version 2 is deterministic, so a file with a given case must be **byte-identical** in every language. For example, `count_min_v2_n1000_cpp.sk` and `count_min_v2_n1000_java.sk` contain the same bytes.

### 8.2 Checks

A reader checks each file from another language as follows:
- It deserializes with the file's seed and reports serial version 2, `numHashes` 3 and `numBuckets` 1000.
- It is empty if and only if `N` = 0.
- The total weight is `N × (N + 1) / 2 + N`.
- For each `i` from 0 to `N - 1`, the estimate for integer `i` is at least `i + 1`, and the estimate for the string of `i` is at least 1. All weights are positive, so a Count-Min sketch never underestimates. Each estimate is also at most its upper bound.
- If the reader's own language has generated the same case, the two files are equal byte for byte. This is the strongest check, because it covers the row seeds, item encoding and bucket mapping at once.

### 8.3 Version 1 files

The existing files `count_min_empty_cpp.sk` and `count_min_non_empty_cpp.sk` are version 1 images. Readers may check that they deserialize as version 1 sketches, but shouldn't compare their estimates across languages, because version 1 hashing differs between languages. Version 2 files always use the `count_min_v2_` prefix, so these names are never reused for version 2 images.
