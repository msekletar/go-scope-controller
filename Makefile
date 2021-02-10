build:
	go build

install: build
	install -o root -g root go-scope-controller /usr/bin
	install -o root -g root init/go-scope-controller.service /usr/lib/systemd/system
	install -o root -g root dbus/com.example.goscopecontroller1.conf /usr/share/dbus-1/system.d/
	systemctl reload dbus-broker.service
	systemctl daemon-reload

uninstall:
	-systemctl stop go-scope-controller.service
	rm -f /usr/bin/go-scope-controller
	rm -f /usr/lib/systemd/system/go-scope-controller.service
	rm -f /usr/share/dbus-1/system.d/com.example.goscopecontroller1.conf
	systemctl reload dbus-broker.service
	systemctl daemon-reload
