fn main() {
    let proto = "../../../api/proto/noteboard/v1/noteboard.proto";
    println!("cargo:rerun-if-changed={proto}");
    // protox — компилятор proto на Rust: не нужен protoc в системе.
    let descriptors = protox::compile([proto], ["../../../api/proto"]).expect("compile proto");
    prost_build::Config::new()
        .compile_fds(descriptors)
        .expect("generate rust");
}
