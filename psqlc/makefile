
make:
	cc -fPIC -I/usr/include/postgresql/12/server -c func.c
	cc -shared -o func.so func.o

clean:
	rm func.so func.o