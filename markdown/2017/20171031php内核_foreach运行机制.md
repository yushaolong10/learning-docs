###php5.6源码分析-foreach
####引言
首先我们先了解一下php的执行流程：
**语法分析(flex)-->词法分析(bison)-->opcode->执行**
######1.词法分析/Zend/zend_language_scanner.l
> 全文搜索foreach，在1090行： 返回token: T_FOREACH
>```php
 <ST_IN_SCRIPTING>"foreach" {
	return T_FOREACH;
}
```

###### 2.语法分析/Zend/zend_language_parser.y
> 全文搜索T_FOREACH, 分别在342及346行： 命中了两种规则，我们选择342行进行分析
> ```php
> |	T_FOREACH '(' variable T_AS
		{ zend_do_foreach_begin(&$1, &$2, &$3, &$4, 1 TSRMLS_CC); }
		foreach_variable foreach_optional_arg ')' { zend_do_foreach_cont(&$1, &$2, &$4, &$6, &$7 TSRMLS_CC); }
		foreach_statement { zend_do_foreach_end(&$1, &$4 TSRMLS_CC); }
```

现在我们重点关注函数: 
- `zend_do_foreach_begin` 
- `zend_do_foreach_cont` 
- `zend_do_foreach_end`

####预备知识点
首先我们先明确几个将要遇到的知识点：
######1.数据结构
*znode*:  编译中bison类型的指定：zend_language_parser.y文件43行`#define YYSTYPE znode`
```php
typedef struct _znode { /* used only during compilation */
	int op_type;
	union {
		znode_op op;
		zval constant; /* replaced by literal/zv */
		zend_op_array *op_array;
		zend_ast *ast;
	} u;
	zend_uint EA;      /* extended attributes */
} znode;
```
*znode_op:*
```php
typedef union _znode_op {
	zend_uint      constant;
	zend_uint      var;
	zend_uint      num;
	zend_ulong     hash;
	zend_uint      opline_num; /*  Needs to be signed */
	zend_op       *jmp_addr;
	zval          *zv;
	zend_literal  *literal;
	void          *ptr;        /* Used for passing pointers from the compile to execution phase, currently used for traits */
} znode_op;

```
*zend_op*:opcode数据结构
```php
struct _zend_op {
	opcode_handler_t handler;
	znode_op op1;
	znode_op op2;
	znode_op result;
	ulong extended_value;
	uint lineno;
	zend_uchar opcode;
	zend_uchar op1_type;
	zend_uchar op2_type;
	zend_uchar result_type;
};
```
*zend_op_array*:用于存放opcode数组
```php
struct _zend_op_array {
	/* Common elements */
	zend_uchar type;
	const char *function_name;
	zend_class_entry *scope;
	zend_uint fn_flags;
	union _zend_function *prototype;
	zend_uint num_args;
	zend_uint required_num_args;
	zend_arg_info *arg_info;
	/* END of common elements */

	zend_uint *refcount;

	zend_op *opcodes;
	zend_uint last;

	zend_compiled_variable *vars;
	int last_var;

	zend_uint T;

	zend_uint nested_calls;
	zend_uint used_stack;

	zend_brk_cont_element *brk_cont_array;
	int last_brk_cont;

	zend_try_catch_element *try_catch_array;
	int last_try_catch;
	zend_bool has_finally_block;

	/* static variables support */
	HashTable *static_variables;

	zend_uint this_var;

	const char *filename;
	zend_uint line_start;
	zend_uint line_end;
	const char *doc_comment;
	zend_uint doc_comment_len;
	zend_uint early_binding; /* the linked list of delayed declarations */

	zend_literal *literals;
	int last_literal;

	void **run_time_cache;
	int  last_cache_slot;

	void *reserved[ZEND_MAX_RESERVED_RESOURCES];
};
```

- - -

######2.宏定义
CG():主要做了对于底层数据的读取的封装，兼容了线程安全和非线程安全 TSRM(Thread safe resource manage)
```php
/* Compiler */
#ifdef ZTS
# define CG(v) TSRMG(compiler_globals_id, zend_compiler_globals *, v)
int zendparse(void *compiler_globals);
#else
# define CG(v) (compiler_globals.v)
extern ZEND_API struct _zend_compiler_globals compiler_globals;
int zendparse(void);
#endif
```

EX():程序运行时data管理
```php
#define EX(element) execute_data->element
```
EX_CV():
```php
#define EX_CV(var)   (*EX_CV_NUM(execute_data, var))

#define EX_CV_NUM(ex, n)       (((zval***)(((char*)(ex))+ZEND_MM_ALIGNED_SIZE(sizeof(zend_execute_data))))+(n))

```
EG():获取程序运行时，全局变量的成员值
```php
/* Executor */
#ifdef ZTS
# define EG(v) TSRMG(executor_globals_id, zend_executor_globals *, v)
#else
# define EG(v) (executor_globals.v)
extern ZEND_API zend_executor_globals executor_globals;
#endif
```
SET_NODE():
```php
#define SET_NODE(target, src) do { \
		target ## _type = (src)->op_type; \
		if ((src)->op_type == IS_CONST) { \
			target.constant = zend_add_literal(CG(active_op_array), &(src)->u.constant TSRMLS_CC); \
		} else { \
			target = (src)->u.op; \
		} \
	} while (0)
```
SET_UNUSED():
```php
#define SET_UNUSED(op)  op ## _type = IS_UNUSED
```
COPY_NODE():
```php
#define COPY_NODE(target, src) do { \
		target ## _type = src ## _type; \
		target = src; \
	} while (0)
```
GET_NODE():
```php
#define GET_NODE(target, src) do { \
		(target)->op_type = src ## _type; \
		if ((target)->op_type == IS_CONST) { \
			(target)->u.constant = CONSTANT(src.constant); \
		} else { \
			(target)->u.op = src; \
			(target)->EA = 0; \
		} \
	} while (0)
```
######3.函数库列表

```php
序号1：是否是函数或者方法回调
static inline zend_bool zend_is_function_or_method_call(const znode *variable) /* {{{ */
{
	zend_uint type = variable->EA;

	return  ((type & ZEND_PARSED_METHOD_CALL) || (type == ZEND_PARSED_FUNCTION_CALL));
}

序号2：
void zend_do_end_variable_parse(znode *variable, int type, int arg_offset TSRMLS_DC) /* {{{ */
{
	zend_llist *fetch_list_ptr;
	zend_llist_element *le;
	zend_op *opline = NULL;
	zend_op *opline_ptr;
	zend_uint this_var = -1;

	zend_stack_top(&CG(bp_stack), (void **) &fetch_list_ptr);

	le = fetch_list_ptr->head;

	/* TODO: $foo->x->y->z = 1 should fetch "x" and "y" for R or RW, not just W */

	if (le) {
		opline_ptr = (zend_op *)le->data;
		if (opline_is_fetch_this(opline_ptr TSRMLS_CC)) {
			/* convert to FETCH_?(this) into IS_CV */
			if (CG(active_op_array)->last == 0 ||
			    CG(active_op_array)->opcodes[CG(active_op_array)->last-1].opcode != ZEND_BEGIN_SILENCE) {

				this_var = opline_ptr->result.var;
				if (CG(active_op_array)->this_var == -1) {
					CG(active_op_array)->this_var = lookup_cv(CG(active_op_array), Z_STRVAL(CONSTANT(opline_ptr->op1.constant)), Z_STRLEN(CONSTANT(opline_ptr->op1.constant)), Z_HASH_P(&CONSTANT(opline_ptr->op1.constant)) TSRMLS_CC);
					Z_TYPE(CONSTANT(opline_ptr->op1.constant)) = IS_NULL;
				} else {
					zend_del_literal(CG(active_op_array), opline_ptr->op1.constant);
				}
				le = le->next;
				if (variable->op_type == IS_VAR &&
				    variable->u.op.var == this_var) {
					variable->op_type = IS_CV;
					variable->u.op.var = CG(active_op_array)->this_var;
				}
			} else if (CG(active_op_array)->this_var == -1) {
				CG(active_op_array)->this_var = lookup_cv(CG(active_op_array), estrndup("this", sizeof("this")-1), sizeof("this")-1, THIS_HASHVAL TSRMLS_CC);
			}
		}

		while (le) {
			opline_ptr = (zend_op *)le->data;
			if (opline_ptr->opcode == ZEND_SEPARATE) {
				if (type != BP_VAR_R && type != BP_VAR_IS) {
					opline = get_next_op(CG(active_op_array) TSRMLS_CC);
					memcpy(opline, opline_ptr, sizeof(zend_op));
				}
				le = le->next;
				continue;
			}
			opline = get_next_op(CG(active_op_array) TSRMLS_CC);
			memcpy(opline, opline_ptr, sizeof(zend_op));
			if (opline->op1_type == IS_VAR &&
			    opline->op1.var == this_var) {
				opline->op1_type = IS_CV;
				opline->op1.var = CG(active_op_array)->this_var;
			}
			switch (type) {
				case BP_VAR_R:
					if (opline->opcode == ZEND_FETCH_DIM_W && opline->op2_type == IS_UNUSED) {
						zend_error_noreturn(E_COMPILE_ERROR, "Cannot use [] for reading");
					}
					opline->opcode -= 3;
					break;
				case BP_VAR_W:
					break;
				case BP_VAR_RW:
					opline->opcode += 3;
					break;
				case BP_VAR_IS:
					if (opline->opcode == ZEND_FETCH_DIM_W && opline->op2_type == IS_UNUSED) {
						zend_error_noreturn(E_COMPILE_ERROR, "Cannot use [] for reading");
					}
					opline->opcode += 6; /* 3+3 */
					break;
				case BP_VAR_FUNC_ARG:
					opline->opcode += 9; /* 3+3+3 */
					opline->extended_value |= arg_offset;
					break;
				case BP_VAR_UNSET:
					if (opline->opcode == ZEND_FETCH_DIM_W && opline->op2_type == IS_UNUSED) {
						zend_error_noreturn(E_COMPILE_ERROR, "Cannot use [] for unsetting");
					}
					opline->opcode += 12; /* 3+3+3+3 */
					break;
			}
			le = le->next;
		}
		if (opline && type == BP_VAR_W && arg_offset) {
			opline->extended_value |= ZEND_FETCH_MAKE_REF;
		}
	}
	zend_llist_destroy(fetch_list_ptr);
	zend_stack_del_top(&CG(bp_stack));
}

序号3：CG(active_op_array)编译时初始化
void init_compiler(TSRMLS_D) /* {{{ */
{
	CG(active_op_array) = NULL;
	memset(&CG(context), 0, sizeof(CG(context)));
	zend_init_compiler_data_structures(TSRMLS_C);
	zend_init_rsrc_list(TSRMLS_C);
	zend_hash_init(&CG(filenames_table), 5, NULL, (dtor_func_t) free_estring, 0);
	zend_llist_init(&CG(open_files), sizeof(zend_file_handle), (void (*)(void *)) file_handle_dtor, 0);
	CG(unclean_shutdown) = 0;
}

序号4：CG(active_op_array)编译中赋值 Zend\zend_language_scanner.l:
 ZEND_API zend_op_array *compile_file(zend_file_handle *file_handle, int type TSRMLS_DC)
{
	zend_lex_state original_lex_state;
	zend_op_array *op_array = (zend_op_array *) emalloc(sizeof(zend_op_array));
	zend_op_array *original_active_op_array = CG(active_op_array);
	zend_op_array *retval=NULL;
	int compiler_result;
	zend_bool compilation_successful=0;
	znode retval_znode;
	zend_bool original_in_compilation = CG(in_compilation);

	retval_znode.op_type = IS_CONST;
	INIT_PZVAL(&retval_znode.u.constant);
	ZVAL_LONG(&retval_znode.u.constant, 1);

	zend_save_lexical_state(&original_lex_state TSRMLS_CC);

	retval = op_array; /* success oriented */

	if (open_file_for_scanning(file_handle TSRMLS_CC)==FAILURE) {
		if (type==ZEND_REQUIRE) {
			zend_message_dispatcher(ZMSG_FAILED_REQUIRE_FOPEN, file_handle->filename TSRMLS_CC);
			zend_bailout();
		} else {
			zend_message_dispatcher(ZMSG_FAILED_INCLUDE_FOPEN, file_handle->filename TSRMLS_CC);
		}
		compilation_successful=0;
	} else {
		init_op_array(op_array, ZEND_USER_FUNCTION, INITIAL_OP_ARRAY_SIZE TSRMLS_CC); //op_array初始化，，初始化op_codes数量为`#define INITIAL_INTERACTIVE_OP_ARRAY_SIZE 8192`
		CG(in_compilation) = 1;
		CG(active_op_array) = op_array; //active_op_code赋值
		zend_stack_push(&CG(context_stack), (void *) &CG(context), sizeof(CG(context)));
		zend_init_compiler_context(TSRMLS_C);
		compiler_result = zendparse(TSRMLS_C);
		zend_do_return(&retval_znode, 0 TSRMLS_CC);
		CG(in_compilation) = original_in_compilation;
		if (compiler_result != 0) { /* parser error */
			zend_bailout();
		}
		compilation_successful=1;
	}

	if (retval) {
		CG(active_op_array) = original_active_op_array;
		if (compilation_successful) {
			pass_two(op_array TSRMLS_CC);
			zend_release_labels(0 TSRMLS_CC);
		} else {
			efree(op_array);
			retval = NULL;
		}
	}
	zend_restore_lexical_state(&original_lex_state TSRMLS_CC);
	return retval;
}

序号5：获取下一个opcode
zend_op *get_next_op(zend_op_array *op_array TSRMLS_DC)
{
	zend_uint next_op_num = op_array->last++;
	zend_op *next_op;

	if (next_op_num >= CG(context).opcodes_size) {
		if (op_array->fn_flags & ZEND_ACC_INTERACTIVE) {
			/* we messed up */
			zend_printf("Ran out of opcode space!\n"
						"You should probably consider writing this huge script into a file!\n");
			zend_bailout();
		}
		CG(context).opcodes_size *= 4;
		op_array_alloc_ops(op_array, CG(context).opcodes_size);
	}
	
	next_op = &(op_array->opcodes[next_op_num]);//C语言申请空间时分配，opcode=(zend_op *)malloc(sizeof(zend_op)*5),则可以以数组形式opcode[3]，访问不同内存空间指针
	
	init_op(next_op TSRMLS_CC); //见序号6

	return next_op;
}

序号6：初始化op
void init_op(zend_op *op TSRMLS_DC)
{
	memset(op, 0, sizeof(zend_op));//为新申请的内存做初始化工作,赋给op
	op->lineno = CG(zend_lineno);
	SET_UNUSED(op->result);
}

序号7：获取下一个op序号
int get_next_op_number(zend_op_array *op_array)
{
	return op_array->last;
}
```


####foreach函数分析
- zend_do_foreach_begin函数

```php
void zend_do_foreach_begin(znode *foreach_token, znode *open_brackets_token, znode *array, znode *as_token, int variable TSRMLS_DC) /* {{{ */
{
	zend_op *opline;
	zend_bool is_variable;
	zend_op dummy_opline;

	if (variable) {
		if (zend_is_function_or_method_call(array)) {//参见函数列表序号1
			is_variable = 0;#传参array不是变量
		} else {
			is_variable = 1;#传参array是变量
		}
		/* save the location of FETCH_W instruction(s) */
		open_brackets_token->u.op.opline_num = get_next_op_number(CG(active_op_array));
		zend_do_end_variable_parse**(array, BP_VAR_W, 0 TSRMLS_CC);//结束变量解析，参见函数列表序号2

		if (zend_is_function_or_method_call(array)) {
			opline = get_next_op(CG(active_op_array) TSRMLS_CC);//对下一个opcode空间初始化
			opline->opcode = ZEND_SEPARATE;
			SET_NODE(opline->op1, array);//对节点进行赋值array到opcode,并标识op_type
			SET_UNUSED(opline->op2);//标识op_type为unused
			opline->result_type = IS_VAR;
			opline->result.var = opline->op1.var;
		}
	} else {
		is_variable = 0;
		open_brackets_token->u.op.opline_num = get_next_op_number(CG(active_op_array));
	}

	/* save the location of FE_RESET */
	foreach_token->u.op.opline_num = get_next_op_number(CG(active_op_array));

	opline = get_next_op(CG(active_op_array) TSRMLS_CC);

	/* Preform array reset */
	opline->opcode = ZEND_FE_RESET;
	opline->result_type = IS_VAR;
	opline->result.var = get_temporary_variable(CG(active_op_array));
	SET_NODE(opline->op1, array);
	SET_UNUSED(opline->op2);
	opline->extended_value = is_variable ? ZEND_FE_RESET_VARIABLE : 0;

	COPY_NODE(dummy_opline.result, opline->result);//拷贝内存副本
	zend_stack_push(&CG(foreach_copy_stack), (void *) &dummy_opline, sizeof(zend_op));//将副本的值压入foreach 栈

	/* save the location of FE_FETCH */
	as_token->u.op.opline_num = get_next_op_number(CG(active_op_array));

	opline = get_next_op(CG(active_op_array) TSRMLS_CC);//申请新空间操作
	opline->opcode = ZEND_FE_FETCH;
	opline->result_type = IS_VAR;
	opline->result.var = get_temporary_variable(CG(active_op_array));
	COPY_NODE(opline->op1, dummy_opline.result);
	opline->extended_value = 0;
	SET_UNUSED(opline->op2);

	opline = get_next_op(CG(active_op_array) TSRMLS_CC);//申请新空间操作
	opline->opcode = ZEND_OP_DATA;
	SET_UNUSED(opline->op1);
	SET_UNUSED(opline->op2);
	SET_UNUSED(opline->result);
}

```
- zend_do_foreach_cont函数

```php
void zend_do_foreach_cont(znode *foreach_token, const znode *open_brackets_token, const znode *as_token, znode *value, znode *key TSRMLS_DC) /* {{{ */
{
	zend_op *opline;
	znode dummy, value_node;
	zend_bool assign_by_ref=0;

	opline = &CG(active_op_array)->opcodes[as_token->u.op.opline_num];//根据opline.num获取active_op_array中opline值
	if (key->op_type != IS_UNUSED) {//k-v值互换
		znode *tmp;

		/* switch between the key and value... */
		tmp = key;
		key = value;
		value = tmp;

		/* Mark extended_value in case both key and value are being used */
		opline->extended_value |= ZEND_FE_FETCH_WITH_KEY;
	}

	if ((key->op_type != IS_UNUSED)) {//对key操作
		if (key->EA & ZEND_PARSED_REFERENCE_VARIABLE) {
			zend_error_noreturn(E_COMPILE_ERROR, "Key element cannot be a reference");
		}
		if (key->EA & ZEND_PARSED_LIST_EXPR) {
			zend_error_noreturn(E_COMPILE_ERROR, "Cannot use list as key element");
		}
	}

	if (value->EA & ZEND_PARSED_REFERENCE_VARIABLE) {//对value操作
		assign_by_ref = 1;

		/* Mark extended_value for assign-by-reference */
		opline->extended_value |= ZEND_FE_FETCH_BYREF;
		CG(active_op_array)->opcodes[foreach_token->u.op.opline_num].extended_value |= ZEND_FE_RESET_REFERENCE;
	} else {
		zend_op *fetch = &CG(active_op_array)->opcodes[foreach_token->u.op.opline_num];
		zend_op	*end = &CG(active_op_array)->opcodes[open_brackets_token->u.op.opline_num];

		/* Change "write context" into "read context" */
		fetch->extended_value = 0;  /* reset ZEND_FE_RESET_VARIABLE */
		while (fetch != end) {
			--fetch;
			if (fetch->opcode == ZEND_FETCH_DIM_W && fetch->op2_type == IS_UNUSED) {
				zend_error_noreturn(E_COMPILE_ERROR, "Cannot use [] for reading");
			}
			if (fetch->opcode == ZEND_SEPARATE) {
				MAKE_NOP(fetch);
			} else {
				fetch->opcode -= 3; /* FETCH_W -> FETCH_R */
			}
		}
	}

	GET_NODE(&value_node, opline->result);

	if (value->EA & ZEND_PARSED_LIST_EXPR) {
		if (!CG(list_llist).head) {
			zend_error_noreturn(E_COMPILE_ERROR, "Cannot use empty list");
		}
		zend_do_list_end(&dummy, &value_node TSRMLS_CC);
		zend_do_free(&dummy TSRMLS_CC);
	} else {
		if (assign_by_ref) {
			zend_do_end_variable_parse(value, BP_VAR_W, 0 TSRMLS_CC);
			/* Mark FE_FETCH as IS_VAR as it holds the data directly as a value */
			zend_do_assign_ref(NULL, value, &value_node TSRMLS_CC);
		} else {
			zend_do_assign(&dummy, value, &value_node TSRMLS_CC);
			zend_do_free(&dummy TSRMLS_CC);
		}
	}

	if (key->op_type != IS_UNUSED) {
		znode key_node;

		opline = &CG(active_op_array)->opcodes[as_token->u.op.opline_num+1];
		opline->result_type = IS_TMP_VAR;
		opline->result.opline_num = get_temporary_variable(CG(active_op_array));
		GET_NODE(&key_node, opline->result);

		zend_do_assign(&dummy, key, &key_node TSRMLS_CC);
		zend_do_free(&dummy TSRMLS_CC);
	}

	do_begin_loop(TSRMLS_C);
	INC_BPC(CG(active_op_array));
}
/* }}} */
```

- zend_do_foreach_end函数

```php
void zend_do_foreach_end(const znode *foreach_token, const znode *as_token TSRMLS_DC) /* {{{ */
{
	zend_op *container_ptr;
	zend_op *opline = get_next_op(CG(active_op_array) TSRMLS_CC);

	opline->opcode = ZEND_JMP;
	opline->op1.opline_num = as_token->u.op.opline_num;
	SET_UNUSED(opline->op1);
	SET_UNUSED(opline->op2);

	CG(active_op_array)->opcodes[foreach_token->u.op.opline_num].op2.opline_num = get_next_op_number(CG(active_op_array)); /* FE_RESET */
	CG(active_op_array)->opcodes[as_token->u.op.opline_num].op2.opline_num = get_next_op_number(CG(active_op_array)); /* FE_FETCH */

	do_end_loop(as_token->u.op.opline_num, 1 TSRMLS_CC);

	zend_stack_top(&CG(foreach_copy_stack), (void **) &container_ptr);
	generate_free_foreach_copy(container_ptr TSRMLS_CC);
	zend_stack_del_top(&CG(foreach_copy_stack));

	DEC_BPC(CG(active_op_array));
}
/* }}} */
```





















