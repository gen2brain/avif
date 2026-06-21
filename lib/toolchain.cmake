include(/opt/wasi-sdk/share/cmake/wasi-sdk-p1.cmake)

# libaom and dav1d pull in <setjmp.h>, which the wasi-sysroot guards behind
# __wasm_exception_handling__. They never take the setjmp/longjmp path here, so we only need the
# type declarations; define the macro to satisfy the header without emitting any
# exception-handling opcodes (kept scalar/clean for the wasm2go transpiler).
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -D__wasm_exception_handling__")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -D__wasm_exception_handling__")
