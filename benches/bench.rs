use criterion::{criterion_group, criterion_main, Criterion};
use std::path::PathBuf;
use treefmt::config;

pub fn bench_group(c: &mut Criterion) {
    c.bench_function("parse config", |b| {
        b.iter(|| {
            let treefmt_toml = PathBuf::from("./treefmt.toml");
            let root = config::from_path(&treefmt_toml);

            assert!(root.is_ok());
        })
    });
}

criterion_group!(benches, bench_group);
criterion_main!(benches);
