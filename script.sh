#!/bin/bash

# install requirements
apt-get update
apt-get install -y git cmake build-essential cvc4 libboost-all-dev wget g++ clang libc6-dev libclang-dev pkg-config unzip

# install z3
wget https://github.com/Z3Prover/z3/releases/download/z3-4.8.17/z3-4.8.17-x64-glibc-2.31.zip
unzip z3-4.8.17-x64-glibc-2.31.zip
cp z3-4.8.17-x64-glibc-2.31/include/* /usr/include/
cp z3-4.8.17-x64-glibc-2.31/bin/z3 /usr/bin/

# build
git clone https://github.com/ethereum/solidity.git
cd solidity
mkdir build
cd build
cmake .. && make -j 8