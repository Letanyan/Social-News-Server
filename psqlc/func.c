#include "postgres.h"
#include "fmgr.h"

PG_MODULE_MAGIC;

PG_FUNCTION_INFO_V1(c_sumRatioAgg);

Datum c_sumRatioAgg(PG_FUNCTION_ARGS)
{
    double accumulator = PG_GETARG_FLOAT8(0);
    double x = PG_GETARG_FLOAT8(1);
    double y = PG_GETARG_FLOAT8(2);

    double denom = x + y;
    PG_RETURN_FLOAT8(accumulator + x / denom);
}

PG_FUNCTION_INFO_V1(c_sumRatioCombine);

Datum c_sumRatioCombine(PG_FUNCTION_ARGS)
{
    double x = PG_GETARG_FLOAT8(0);
    double y = PG_GETARG_FLOAT8(1);
    PG_RETURN_FLOAT8(x + y);
}

PG_FUNCTION_INFO_V1(c_sumRatioFinal);

Datum c_sumRatioFinal(PG_FUNCTION_ARGS)
{
    double x = PG_GETARG_FLOAT8(0);
    PG_RETURN_FLOAT8(x);
}

// ---------------------------------------------------

typedef struct ScoreValueType
{
    double tagV, userV, countV, factor;
} ScoreValueType;

#define DatumGetScoreValue(X) ((ScoreValueType *)DatumGetPointer(X))
#define ScoreValueGetDatum(X) PointerGetDatum(X)
#define PG_GETARG_SCORE_VALUE(n) DatumGetScoreValue(PG_GETARG_DATUM(n))
#define PG_RETURN_SCORE_VALUE(x) return ScoreValueGetDatum(x)

PG_FUNCTION_INFO_V1(c_scoreValueFactorAgg);

Datum c_scoreValueFactorAgg(PG_FUNCTION_ARGS)
{
    ScoreValueType *cAgg = PG_GETARG_SCORE_VALUE(0);
    float8 tagValue = PG_GETARG_FLOAT8(1);
    float8 userValue = PG_GETARG_FLOAT8(2);
    float8 factor = PG_GETARG_FLOAT8(3);

    // cAgg->tagV = cAgg->tagV + tagValue;
    // cAgg->userV = cAgg->userV + userValue;
    // cAgg->countV = cAgg->countV + 1;
    // cAgg->factor = factor;

    // WHY IS THERE A 'HEADER????' IN THE DATA STRUCTURE FROM THE DB!!!
    // The first 24 bytes shouldn't be accessed as they don't seem to
    // corrospond to anything we know of and our data starts after 24 bytes. 
    double *raw = (double *)cAgg;
    raw[3] = raw[3] + tagValue;
    raw[4] = raw[4] + userValue;
    raw[5] = raw[5] + 1;
    raw[6] = factor;

    PG_RETURN_SCORE_VALUE((ScoreValueType *)cAgg);
}

PG_FUNCTION_INFO_V1(c_scoreValueFactorCombine);

Datum c_scoreValueFactorCombine(PG_FUNCTION_ARGS)
{
    ScoreValueType *cAgg = PG_GETARG_SCORE_VALUE(0);
    ScoreValueType *dAgg = PG_GETARG_SCORE_VALUE(1);

    // cAgg->tagV = cAgg->tagV + dAgg->tagV;
    // cAgg->userV = cAgg->userV + dAgg->userV;
    // cAgg->countV = cAgg->countV + dAgg->countV;
    // cAgg->factor = (cAgg->factor + dAgg->factor) / 2;

    double *rawA = (double *)cAgg;
    double *rawB = (double *)dAgg;
    rawA[3] = rawA[3] + rawB[3];
    rawA[4] = rawA[4] + rawB[4];
    rawA[5] = rawA[5] + rawB[5];
    rawA[6] = (rawA[6] + rawB[6]) / 2;

    PG_RETURN_SCORE_VALUE(cAgg);
}

PG_FUNCTION_INFO_V1(c_scoreValueFactorFinal);

Datum c_scoreValueFactorFinal(PG_FUNCTION_ARGS)
{
    ScoreValueType *cAgg = PG_GETARG_SCORE_VALUE(0);
    double *raw = (double *)cAgg;

    // float8 score = cAgg->tagV + (cAgg->userV / cAgg->countV);
    float8 score = raw[3] + (raw[4] / raw[5]);
    if (score < 0)
    {
        score = 0;
    }

    PG_RETURN_FLOAT8(score * raw[6]);
}