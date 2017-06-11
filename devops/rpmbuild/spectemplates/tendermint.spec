
%define __spec_install_post %{nil}
%define debug_package       %{nil}
%define __os_install_post   %{nil}

Name: tendermint
Summary: securely and consistently replicate an application on many machines
License: Apache 2.0
URL: https://tendermint.com/
Packager: Greg Szabo

%description
Tendermint is software for securely and consistently replicating an application on many machines. By securely, we mean that Tendermint works even if up to 1/3 of machines fail in arbitrary ways. By consistently, we mean that every non-faulty machine sees the same transaction log and computes the same state.

%prep
test -d "$GOPATH" || echo "GOPATH not set"
test -d "$GOPATH"
%{__mkdir_p} %{name}-%{version}
cd %{name}-%{version}
%{__mkdir_p} .%{_bindir} .%{_defaultlicensedir}/%{name}
%{__cp} $GOPATH/bin/tendermint .%{_bindir}
%{__cp} $GOPATH/src/github.com/tendermint/tendermint/LICENSE .%{_defaultlicensedir}/%{name}
%{__chmod} -Rf a+rX,u+w,g-w,o-w .

%build
# Nothing to do here.

%install
cd %{name}-%{version}
%{__cp} -a * %{buildroot}

%files
%{_bindir}/tendermint
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

