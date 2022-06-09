use rust_gpu_tools::Device;
use strum::AsRefStr;

#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq, AsRefStr)]
#[strum(serialize_all = "UPPERCASE")]
pub enum Vendor {
    /// GPU by AMD.
    Amd,
    /// GPU by NVIDIA.
    Nvidia,
}

pub struct GPUInfo {
    pub name: String,
    pub vendor: Vendor,
    pub memory: u64,
}

pub fn load() -> Vec<GPUInfo> {
    Device::all()
        .iter()
        .map(|dev| GPUInfo {
            name: dev.name(),
            vendor: dev.vendor().into(),
            memory: dev.memory(),
        })
        .collect()
}

impl From<rust_gpu_tools::Vendor> for Vendor {
    fn from(rust_gpu_tools_vendor: rust_gpu_tools::Vendor) -> Self {
        match rust_gpu_tools_vendor {
            rust_gpu_tools::Vendor::Amd => Vendor::Amd,
            rust_gpu_tools::Vendor::Nvidia => Vendor::Nvidia,
        }
    }
}
