build *args='':
	cargo build {{args}}

test *args='':
	cargo test {{args}}

coverage *args='':
    mkdir -p target/coverage
    CARGO_INCREMENTAL=0 RUSTFLAGS='-Cinstrument-coverage' LLVM_PROFILE_FILE='cargo-test-%p-%m.profraw' cargo test {{args}}
    grcov . --llvm-path $LLVM_PATH --binary-path ./target/debug/deps/ -s . -t html --branch --ignore-not-existing --ignore '../*' --ignore "/*" -o target/coverage/html
    rm *.profraw # cleanup
    xdg-open ./target/coverage/html/index.html 1>/dev/null 2>&1
