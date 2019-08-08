Vagrant.configure("2") do |config|
  config.vm.provider "virtualbox" do |v|
    v.memory = 2048
  end
  config.vm.box = "debian/buster64"
  config.vm.provision "shell",
    inline: <<-EOS
      apt-get install -y curl git gcc libssl-dev pkg-config mtd-utils
      curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
EOS
end
