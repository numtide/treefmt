use criterion::{criterion_group, criterion_main, Criterion};
use std::path::PathBuf;
use treefmt::config;
use treefmt::engine;

pub fn bench_group(c: &mut Criterion) {
    c.bench_function("parse config", |b| b.iter(|| {
        let treefmt_toml = PathBuf::from("./treefmt.toml");
        let root = config::from_path(&treefmt_toml);

        assert!(root.is_ok());
    }));

    let treefmt_toml = PathBuf::from("./treefmt.toml");
    let root = config::from_path(&treefmt_toml).unwrap();

    c.bench_function("command context", |b| b.iter(|| {
        let dir = PathBuf::from(".");
        let ctx = engine::create_command_context(&dir, &root);

        assert!(ctx.is_ok());
    }));
}

criterion_group!(benches, bench_group);
criterion_main!(benches);
