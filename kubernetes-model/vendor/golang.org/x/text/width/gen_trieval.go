/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

// elem is an entry of the width trie. The high byte is used to encode the type
// of the rune. The low byte is used to store the index to a mapping entry in
// the inverseData array.
type elem uint16

const (
	tagNeutral elem = iota << typeShift
	tagAmbiguous
	tagWide
	tagNarrow
	tagFullwidth
	tagHalfwidth
)

const (
	numTypeBits = 3
	typeShift   = 16 - numTypeBits

	// tagNeedsFold is true for all fullwidth and halfwidth runes except for
	// the Won sign U+20A9.
	tagNeedsFold = 0x1000

	// The Korean Won sign is halfwidth, but SHOULD NOT be mapped to a wide
	// variant.
	wonSign rune = 0x20A9
)
