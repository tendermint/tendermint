Version: @VERSION@
Release: @BUILD_NUMBER@

%define __spec_install_post %{nil}
%define debug_package       %{nil}
%define __os_install_post   %{nil}

Name: @PACKAGE_NAME@
Summary: @PACKAGE_SUMMARY@
License: Apache 2.0
URL: @PACKAGE_URL@
Packager: Greg Szabo
@PACKAGE_ADDITIONAL_HEADER@

%description
@PACKAGE_DESCRIPTION@

%pre
if ! %{__grep} -q '^%{name}:' /etc/passwd ; then
  useradd -r -b %{_sysconfdir} %{name}
  mkdir -p %{_sysconfdir}/%{name}
  chmod 755 %{_sysconfdir}/%{name}
  chown %{name}.%{name} %{_sysconfdir}/%{name}
fi

%prep
# Nothing to do here. - It is done in the Makefile.

%build
# Nothing to do here.

%install
cd %{name}-%{version}-%{release}
%{__cp} -a * %{buildroot}

%post
sudo -Hu %{name} %{name} node init --home %{_sysconfdir}/%{name} 2B24DEE2364762300168DF19B6C18BCE2D399EA2
systemctl daemon-reload

%preun
systemctl stop %{name} 2> /dev/null || :

%postun
systemctl daemon-reload

%files
%ghost %attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}
%{_bindir}/*
%{_sysconfdir}/systemd/system/*
%{_sysconfdir}/systemd/system-preset/*
%dir %{_datadir}/%{name}
%{_datadir}/%{name}/*
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

