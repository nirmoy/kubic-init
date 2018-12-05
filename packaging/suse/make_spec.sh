#!/bin/bash

if [ -z "$1" ]; then
  cat <<EOF
usage:
  ./make_spec.sh PACKAGE [BRANCH]
EOF
  exit 1
fi

cd $(dirname $0)

YEAR=$(date +%Y)
VERSION=$(cat ../../VERSION)
COMMIT_ID=$(git rev-parse HEAD)
COMMIT_INFO=$(git show -s --format=%ct.%h)
VERSION_PKG="${VERSION%+*}+git.${COMMIT_INFO}"
BUILD_DATE=$(date +%Y%m%d-%T)
NAME=$1
BRANCH=${2:-master}
SAFE_BRANCH=${BRANCH//\//-}

cat <<EOF > ${NAME}.spec
#
# spec file for package $NAME
#
# Copyright (c) $YEAR SUSE LINUX GmbH, Nuernberg, Germany.
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via http://bugs.opensuse.org/
#

# Project name when using go tooling.
%define go_version 1.11
%define project github.com/kubic-project/kubic-init

Name:           $NAME
Version:        $VERSION_PKG
Release:        0
Summary:        Simplify the process of creating and maintaining Kubernetes clusters
License:        Apache-2.0
Group:          System/Management
Url:            https://github.com/kubic-project/kubic-init/archive/master.tar.gz
Source0:        ${SAFE_BRANCH}.tar.gz
Source1:        ${NAME}-vendor.tar.gz
BuildRequires:  go >= %{go_version}
BuildRequires:  golang-packaging
BuildRequires:  golang(API) = %{go_version}
BuildRoot:      %{_tmppath}/%{name}-%{version}-build
Requires:       kubernetes-kubeadm
%{go_nostrip}
%{go_provides}

%description
The Kubic-init repository is a project geared at simplifying the process of creating and maintaining kubernetes clusters. Our goal is to make the process as simple as possible using existing tools and developing new ones.

%prep
%setup -q -b 0 -n ${NAME}-${SAFE_BRANCH}
%setup -q -b 1 -n ${NAME}-${SAFE_BRANCH}

%build
%{goprep} github.com/kubic-project/kube-init
export GOPATH=\$HOME/go
mkdir -pv \$HOME/go/src/%{project}
rm -rf \$HOME/go/src/%{project}/*
cp -avr * \$HOME/go/src/%{project}

export GO_VERSION=%{go_version}

export KUBIC_INIT_VERSION=${VERSION}
export KUBIC_INIT_BUILD=${COMMIT_ID}
export KUBIC_INIT_BRANCH=${SAFE_BRANCH}
export KUBIC_INIT_BUILD_DATE=${BUILD_DATE}
export KUBIC_INIT_EXE="cmd/kubic-init/kubic-init"
export KUBIC_INIT_MAIN="cmd/kubic-init/main.go"

cd \$HOME/go/src/%{project}

env GO111MODULE=off go build -ldflags "-X=main.Version=\${KUBIC_INIT_VERSION} \\
                   -X=main.Build=\${KUBIC_INIT_BUILD} \\
                   -X=main.BuildDate=\${KUBIC_INIT_BUILD_DATE} \\
                   -X=main.Branch=\${KUBIC_INIT_BRANCH} \\
                   -X=main.GoVersion=\${GO_VERSION}" \\
         -o \${KUBIC_INIT_EXE} \${KUBIC_INIT_MAIN}

%install
cd \$HOME/go/src/%{project}
install -D -m 0755 cmd/kubic-init/kubic-init %{buildroot}/%{_bindir}/kubic-init

%files
%defattr(-,root,root)
%license LICENSE
%doc README.md
%{_bindir}/kubic-init

%changelog
EOF
