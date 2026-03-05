sha256sum /home/ubuntu/mupdf/build/release/libmupdf.a \
          /home/ubuntu/go-fitz/libs/libmupdf_$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').a

sha256sum /home/ubuntu/mupdf/build/release/libmupdf-third.a \
          /home/ubuntu/go-fitz/libs/libmupdfthird_$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').a
