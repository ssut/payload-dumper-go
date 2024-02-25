#ifndef CPP_LOGGING_H_
#define CPP_LOGGING_H_

#include <iostream>
#include <cassert>
#include <cstring>
#include <cstdint>
#include <cmath>

//#include "base/template_util.h"

#ifndef DCHECK_IS_ON
#define DCHECK_IS_ON() 1
#endif

#ifndef EAT_STREAM_PARAMETERS
#define EAT_STREAM_PARAMETERS
#endif


#ifndef NOTREACHED
#define NOTREACHED() DCHECK(false)
#endif

#ifndef ANALYZER_SKIP_THIS_PATH
#define ANALYZER_SKIP_THIS_PATH()
#endif

#ifndef DISALLOW_IMPLICIT_CONSTRUCTORS
#define DISALLOW_IMPLICIT_CONSTRUCTORS(...)
#endif

#ifndef LOG
#define LOG(level) std::cout << #level << ": "
#endif

#ifndef LOG_IF
#define LOG_IF(level, cond) if (cond) {} else LOG(level)
#endif

#ifndef DLOG_IF
#define DLOG_IF(level, cond) if (cond) {} else LOG(level)
#endif

#ifndef CHECK
#define CHECK(...) if (__VA_ARGS__) {} else std::cout << "Check failed"
#endif

#ifndef PCHECK
#define PCHECK CHECK
#endif

#ifndef DPCHECK
#define DPCHECK CHECK
#endif

#ifndef DLOG
#define DLOG(level) LOG(level)
#endif

#ifndef VLOG
#define VLOG(level) LOG(level)
#endif

#ifndef PLOG
#define PLOG(level) LOG(level)
#endif

#ifndef DVLOG
#define DVLOG(level) LOG(level)
#endif

#ifndef DPLOG
#define DPLOG(level) LOG(level)
#endif

#ifndef DVPLOG
#define DVPLOG(level) LOG(level)
#endif

#ifndef VPLOG
#define VPLOG(level) LOG(level)
#endif

#ifndef RAW_CHECK
#define RAW_CHECK(...) assert(__VA_ARGS__)
#endif

#ifndef RAW_LOG
#define RAW_LOG(level, ...) LOG(level) << __VA_ARGS__
#endif

#ifndef DCHECK
#define DCHECK CHECK
#endif

#ifndef DCHECK_GT
#define DCHECK_GT CHECK_GT
#endif

#ifndef DCHECK_GE
#define DCHECK_GE CHECK_GE
#endif

#ifndef DCHECK_LT
#define DCHECK_LT CHECK_LT
#endif

#ifndef DCHECK_LE
#define DCHECK_LE CHECK_LE
#endif

#ifndef DCHECK_NE
#define DCHECK_NE CHECK_NE
#endif

#ifndef DCHECK_EQ
#define DCHECK_EQ CHECK_EQ
#endif

#ifndef CHECK_OP
#define CHECK_OP(a, b, op) CHECK((a) op (b))
#endif


#ifndef CHECK_GT
#define CHECK_GT(a, b) CHECK_OP(a, b, >)
#endif

#ifndef CHECK_GE
#define CHECK_GE(a, b) CHECK_OP(a, b, >=)
#endif

#ifndef CHECK_LT
#define CHECK_LT(a, b) CHECK_OP(a, b, <)
#endif

#ifndef CHECK_LE
#define CHECK_LE(a, b) CHECK_OP(a, b, <=)
#endif

#ifndef CHECK_NE
#define CHECK_NE(a, b) CHECK_OP(a, b, !=)
#endif

#ifndef CHECK_EQ
#define CHECK_EQ(a, b) CHECK_OP(a, b, ==)
#endif

#endif /* CPP_LOGGING_H_ */
