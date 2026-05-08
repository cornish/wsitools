// shim.cpp — C wrappers around OpenJPH for cgo.
// All OpenJPH C++ state lives inside this file; cgo only sees `extern "C"`
// declarations.

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <exception>

#include <openjph/ojph_codestream.h>
#include <openjph/ojph_params.h>
#include <openjph/ojph_mem.h>
#include <openjph/ojph_file.h>
#include <openjph/ojph_arch.h>
#include <openjph/ojph_message.h>

extern "C" {

// wsi_htj2k_encode encodes RGB888 (3 components, 8-bit each) as a J2K
// codestream (no JP2 boxing). Uses OpenJPH's "high-throughput" tier when
// available; falls back to standard JPEG 2000 otherwise.
//
// quality: 1..100 (mapped to a quantization step internally; 100 = lossless).
// On success, *outbuf is malloc'd; caller frees.
// Returns 0 on success, -1 on error.
//
// Critical ordering note: codestream::close() invokes outfile::close(),
// which on mem_outfile deallocates the underlying buffer and resets
// cur_ptr. So we MUST read out.tell() and copy out.get_data() BEFORE
// calling cs.close() — otherwise we observe an empty output.
int wsi_htj2k_encode(
    const unsigned char *rgb, int width, int height,
    int quality,
    unsigned char **outbuf, size_t *outsize)
{
    *outbuf = NULL;
    *outsize = 0;

    using namespace ojph;

    try {
        codestream cs;

        param_siz siz = cs.access_siz();
        siz.set_image_extent(point((ui32)width, (ui32)height));
        siz.set_num_components(3);
        for (ui32 c = 0; c < 3; ++c) {
            siz.set_component(c, point(1, 1), 8, false);
        }
        siz.set_image_offset(point(0, 0));
        siz.set_tile_size(size((ui32)width, (ui32)height));
        siz.set_tile_offset(point(0, 0));

        param_cod cod = cs.access_cod();
        cod.set_num_decomposition(5);
        cod.set_block_dims(64, 64);
        cod.set_progression_order("RPCL");
        cod.set_color_transform(true);
        const bool reversible = (quality >= 100);
        cod.set_reversible(reversible);

        if (!reversible) {
            // qstep maps from --quality 1..99 to a quantization step.
            // 0.001 (high quality) to 0.1 (low quality), linear in [1..99].
            float qstep = 0.001f + 0.099f * (float)(100 - quality) / 99.0f;
            cs.access_qcd().set_irrev_quant(qstep);
        }

        // set_planar(false): interleaved exchange — for each row, push
        // component 0, then 1, then 2 before moving to the next row.
        // This is required when color transform (RCT/ICT) is active.
        cs.set_planar(false);

        mem_outfile out;
        out.open();
        cs.write_headers(&out);

        // Line-by-line exchange. First call: line == NULL gets the first
        // buffer; exchange() returns the next line_buf to fill and writes
        // the component index into next_comp.
        ui32 next_comp = 0;
        line_buf *cur = cs.exchange(NULL, next_comp);
        for (int y = 0; y < height; ++y) {
            for (int c = 0; c < 3; ++c) {
                if (!cur || !cur->i32) return -1;
                si32 *target = cur->i32;
                const unsigned char *src = rgb + (size_t)y * width * 3 + c;
                for (int x = 0; x < width; ++x) {
                    target[x] = (si32)src[x * 3];
                }
                cur = cs.exchange(cur, next_comp);
            }
        }

        cs.flush();

        si64 out_size64 = out.tell();
        if (out_size64 <= 0) {
            cs.close();
            return -1;
        }
        size_t out_size = (size_t)out_size64;
        *outbuf = (unsigned char *)malloc(out_size);
        if (!*outbuf) {
            cs.close();
            return -1;
        }
        memcpy(*outbuf, out.get_data(), out_size);
        *outsize = out_size;
        cs.close();
        return 0;
    } catch (const std::exception&) {
        if (*outbuf) { free(*outbuf); *outbuf = NULL; *outsize = 0; }
        return -1;
    } catch (...) {
        if (*outbuf) { free(*outbuf); *outbuf = NULL; *outsize = 0; }
        return -1;
    }
}

}  // extern "C"
