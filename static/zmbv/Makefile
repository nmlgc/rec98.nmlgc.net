.POSIX:
.SUFFIXES:

CC=emcc
RM=rm

all: clean zmbv.wasm

zmbv.wasm: miniz.h miniz.c zmbv.h zmbv.cpp decoder.h decoder.cpp
	$(CC) -O3 -flto -fno-rtti -fno-exceptions --no-entry \
		-sIMPORTED_MEMORY \
		-sEXPORTED_FUNCTIONS=_malloc \
		-o zmbv.wasm \
		miniz.c zmbv.cpp decoder.cpp

clean:
	$(RM) -f zmbv.wasm

.PHONY: all clean
