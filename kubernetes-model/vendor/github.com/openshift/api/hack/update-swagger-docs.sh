#!/bin/bash
#
# Copyright (C) 2015 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#


source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

# Generates types_swagger_doc_generated file for the given group version.
# $1: Name of the group version
# $2: Path to the directory where types.go for that group version exists. This
# is the directory where the file will be generated.
kube::swagger::gen_types_swagger_doc() {
  local group_version=$1
  local gv_dir=$2
  local TMPFILE="${TMPDIR:-/tmp}/types_swagger_doc_generated.$(date +%s).go"

  echo "Generating swagger type docs for ${group_version} at ${gv_dir}"

  sed 's/YEAR/2017/' hack/empty.txt > "$TMPFILE"
  echo "package ${group_version##*/}" >> "$TMPFILE"
  cat >> "$TMPFILE" <<EOF

// This file contains a collection of methods that can be used from go-restful to
// generate Swagger API documentation for its models. Please read this PR for more
// information on the implementation: https://github.com/emicklei/go-restful/pull/215
//
// TODOs are ignored from the parser (e.g. TODO(andronat):... || TODO:...) if and only if
// they are on one line! For multiple line or blocks that you want to ignore use ---.
// Any context after a --- is ignored.
//
// Those methods can be generated by using hack/update-generated-swagger-docs.sh

// AUTO-GENERATED FUNCTIONS START HERE
EOF

  go run tools/genswaggertypedocs/swagger_type_docs.go -s \
    "${gv_dir}/types.go" \
    -f - \
    >>  "$TMPFILE"

  echo "// AUTO-GENERATED FUNCTIONS END HERE" >> "$TMPFILE"

  gofmt -w -s "$TMPFILE"
  mv "$TMPFILE" ""${gv_dir}"/types_swagger_doc_generated.go"
}

for gv in ${API_GROUP_VERSIONS}; do
  rm -f "${SCRIPT_ROOT}/${gv}/types_swagger_doc_generated.go"
  kube::swagger::gen_types_swagger_doc "${gv}" "${gv}"
done
