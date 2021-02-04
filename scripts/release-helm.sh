#!/bin/bash
#
# This file is open source software, licensed to you under the terms
# of the Apache License, Version 2.0 (the "License").  See the NOTICE file
# distributed with this work for additional information regarding copyright
# ownership.  You may not use this file except in compliance with the License.
#
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

set -euo pipefail


if [[ "$CHANNEL" == "nightly" ]]; then
  APP_VERSION="${APP_VERSION:-"nightly"}"
fi

HELM_LOCAL_REPO="helm/repo/${CHANNEL}"

HELM_BUCKET="gs://scylla-operator-charts/${CHANNEL}"
HELM_REPOSITORY="https://scylla-operator-charts.storage.googleapis.com/${CHANNEL}"

DEV_HELM_BUCKET="gs://scylla-operator-charts-dev/${CHANNEL}"
DEV_HELM_REPOSITORY="https://scylla-operator-charts-dev.storage.googleapis.com/${CHANNEL}"

if [[ "${DEV_REPO:-"false"}" == "true" ]]; then
  HELM_REPOSITORY=${DEV_HELM_REPOSITORY}
  HELM_BUCKET=${DEV_HELM_BUCKET}
fi

cd "$(dirname "${BASH_SOURCE[0]}")/.."
set -x

mkdir -p ${HELM_LOCAL_REPO}
gsutil rsync -d ${HELM_BUCKET} ${HELM_LOCAL_REPO}

CHARTS_TO_PUBLISH=(
  scylla-operator
  scylla-manager
  scylla
)

for CHART in "${CHARTS_TO_PUBLISH[@]}"; do
  helm package \
    helm/${CHART} \
    --destination ${HELM_LOCAL_REPO} \
    --app-version ${APP_VERSION} \
    --version ${CHART_VERSION}
done

helm repo index ${HELM_LOCAL_REPO} \
  --url ${HELM_REPOSITORY} \
  --merge ${HELM_LOCAL_REPO}/index.yaml

gsutil rsync -d ${HELM_LOCAL_REPO} ${HELM_BUCKET}
