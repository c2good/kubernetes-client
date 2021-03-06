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

os::log::info "===== Verifying API Swagger Spec ====="

SPECROOT="${OS_ROOT}/api"
REL_TMP_PATH="_output/verify-generated-swagger-spec"
TMP_SPECROOT="${OS_ROOT}/${REL_TMP_PATH}/api"

function cleanup() {
	local return_code="$?"
	rm -rf "${TMP_SPECROOT}"

	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}

trap cleanup EXIT

os::log::info "Generating a fresh spec..."
if ! output=$("${OS_ROOT}/hack/update-generated-swagger-spec.sh" "${REL_TMP_PATH}" 2>&1); then
	os::log::fatal "Generation of fresh spec failed:
${output}"
fi

# TODO: more effective way to do this, or have readme be generated
cp "${SPECROOT}/README.md" "${TMP_SPECROOT}"

os::log::info "Diffing current spec against freshly generated spec..."
if ! output=$(diff -Naupr -I 'Auto generated by' "${SPECROOT}" "${TMP_SPECROOT}" 2>&1); then
	os::log::fatal "Swagger spec is out of date. Please run hack/update-generated-swagger-spec.sh:
${output}"
else
	os::log::info "Swagger spec up to date."
fi