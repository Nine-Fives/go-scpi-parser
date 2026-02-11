#include "c_wrapper.h"
#include "scpi-parser/libscpi/inc/scpi/scpi.h"
#include <string.h>

// --- Output capture ---

#define OUTPUT_BUF_SIZE 4096
static char output_buf[OUTPUT_BUF_SIZE];
static size_t output_pos = 0;

// --- Error tracking ---

static int error_count = 0;

// --- Buffers ---

#define INPUT_BUF_SIZE 256
static char input_buffer[INPUT_BUF_SIZE];

#define ERROR_QUEUE_SIZE 17
static scpi_error_t error_queue[ERROR_QUEUE_SIZE];

// --- Interface callbacks ---

static size_t my_write(scpi_t* ctx, const char* data, size_t len) {
    (void)ctx;
    if (output_pos + len < OUTPUT_BUF_SIZE) {
        memcpy(output_buf + output_pos, data, len);
        output_pos += len;
    }
    return len;
}

static int my_error(scpi_t* ctx, int_fast16_t err) {
    (void)ctx;
    (void)err;
    error_count++;
    return 0;
}

static scpi_result_t my_flush(scpi_t* ctx) {
    (void)ctx;
    return SCPI_RES_OK;
}

static scpi_result_t my_reset(scpi_t* ctx) {
    (void)ctx;
    return SCPI_RES_OK;
}

static scpi_result_t my_control(scpi_t* ctx, scpi_ctrl_name_t ctrl, scpi_reg_val_t val) {
    (void)ctx;
    (void)ctrl;
    (void)val;
    return SCPI_RES_OK;
}

// --- Choice definitions ---

static const scpi_choice_def_t test_choices[] = {
    {"LOW", 0},
    {"MEDium", 1},
    {"HIGH", 2},
    SCPI_CHOICE_LIST_END
};

// --- Command callbacks ---

static scpi_result_t cb_echo_int32(scpi_t* ctx) {
    int32_t val;
    if (!SCPI_ParamInt32(ctx, &val, TRUE)) {
        return SCPI_RES_ERR;
    }
    SCPI_ResultInt32(ctx, val);
    return SCPI_RES_OK;
}

static scpi_result_t cb_echo_double(scpi_t* ctx) {
    double val;
    if (!SCPI_ParamDouble(ctx, &val, TRUE)) {
        return SCPI_RES_ERR;
    }
    SCPI_ResultDouble(ctx, val);
    return SCPI_RES_OK;
}

static scpi_result_t cb_echo_bool(scpi_t* ctx) {
    scpi_bool_t val;
    if (!SCPI_ParamBool(ctx, &val, TRUE)) {
        return SCPI_RES_ERR;
    }
    SCPI_ResultBool(ctx, val);
    return SCPI_RES_OK;
}

static scpi_result_t cb_echo_string(scpi_t* ctx) {
    char buf[256];
    size_t len;
    if (!SCPI_ParamCopyText(ctx, buf, sizeof(buf), &len, TRUE)) {
        return SCPI_RES_ERR;
    }
    SCPI_ResultText(ctx, buf);
    return SCPI_RES_OK;
}

static scpi_result_t cb_echo_choice(scpi_t* ctx) {
    int32_t val;
    if (!SCPI_ParamChoice(ctx, test_choices, &val, TRUE)) {
        return SCPI_RES_ERR;
    }
    SCPI_ResultInt32(ctx, val);
    return SCPI_RES_OK;
}

static scpi_result_t cb_echo_arb(scpi_t* ctx) {
    const char* data;
    size_t len;
    if (!SCPI_ParamArbitraryBlock(ctx, &data, &len, TRUE)) {
        return SCPI_RES_ERR;
    }
    SCPI_ResultArbitraryBlock(ctx, data, len);
    return SCPI_RES_OK;
}

static scpi_result_t cb_noop(scpi_t* ctx) {
    (void)ctx;
    return SCPI_RES_OK;
}

static scpi_result_t cb_query_fixed(scpi_t* ctx) {
    SCPI_ResultInt32(ctx, 42);
    SCPI_ResultDouble(ctx, 3.14);
    SCPI_ResultText(ctx, "hello");
    return SCPI_RES_OK;
}

// --- Command table ---

static const scpi_command_t commands[] = {
    { .pattern = "TEST:INT32",      .callback = cb_echo_int32 },
    { .pattern = "TEST:INT32?",     .callback = cb_echo_int32 },
    { .pattern = "TEST:DOUBle",     .callback = cb_echo_double },
    { .pattern = "TEST:DOUBle?",    .callback = cb_echo_double },
    { .pattern = "TEST:BOOL",       .callback = cb_echo_bool },
    { .pattern = "TEST:BOOL?",      .callback = cb_echo_bool },
    { .pattern = "TEST:TEXT",       .callback = cb_echo_string },
    { .pattern = "TEST:TEXT?",      .callback = cb_echo_string },
    { .pattern = "TEST:CHOice?",    .callback = cb_echo_choice },
    { .pattern = "TEST:ARBitrary?", .callback = cb_echo_arb },
    { .pattern = "TEST:NOOP",       .callback = cb_noop },
    { .pattern = "TEST:QUERy?",     .callback = cb_query_fixed },
    { .pattern = "TEST#:NUMbers#",  .callback = cb_noop },
    SCPI_CMD_LIST_END
};

static scpi_interface_t iface = {
    .error = my_error,
    .write = my_write,
    .control = my_control,
    .flush = my_flush,
    .reset = my_reset,
};

static scpi_t ctx;

// --- Public API ---

void c_scpi_init(void) {
    SCPI_Init(&ctx, commands, &iface, scpi_units_def,
              "FUZZ", "INST", "0", "1.0",
              input_buffer, INPUT_BUF_SIZE,
              error_queue, ERROR_QUEUE_SIZE);
}

const char* c_scpi_input(const char* data, int len, int* out_len, int* err_count) {
    output_pos = 0;
    error_count = 0;
    memset(output_buf, 0, OUTPUT_BUF_SIZE);
    SCPI_Input(&ctx, data, len);
    output_buf[output_pos] = '\0';
    *out_len = (int)output_pos;
    *err_count = error_count;
    return output_buf;
}

void c_scpi_reset(void) {
    c_scpi_init();
}
